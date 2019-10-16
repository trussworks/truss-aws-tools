package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/iam"

	flag "github.com/jessevdk/go-flags"
	"github.com/lytics/slackhook"
	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/internal/aws/ssm"
	"go.uber.org/zap"
)

func parseTimestamp(s string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05-07:00", s)
}

type stringSet map[string]struct{}

func (s stringSet) len() int {
	return len(s)
}

func (s stringSet) has(key string) bool {
	_, ok := s[key]
	return ok
}

func (s stringSet) add(key string) {
	s[key] = struct{}{}
}

func (s stringSet) toArray(sortValues bool) []string {
	arr := sort.StringSlice(make([]string, 0, len(s)))
	for k := range s {
		arr = append(arr, k)
	}
	if sortValues {
		arr.Sort()
	}
	return arr
}

func newstringSet() stringSet {
	return stringSet(map[string]struct{}{})
}

// Options are the command line options
type Options struct {
	DocumentationURL   string `short:"d" long:"documentation-url" description:"URL pointing to documentation on how-to rotate AWS access keys" required:"false" env:"DOCUMENTATION_URL"`
	Profile            string `short:"p" long:"profile" description:"The AWS profile to use." required:"false" env:"AWS_PROFILE"`
	Region             string `long:"region" description:"The AWS region to use." required:"false" env:"REGION"`
	Lambda             bool   `short:"l" long:"lambda" description:"Run as an AWS lambda function." required:"false" env:"LAMBDA"`
	MaxDays            uint   `long:"days" description:"The maximum age in days that a key can be active without triggering an alert." default:"90" env:"MAX_DAYS"`
	PollInterval       uint   `long:"poll-interval" description:"The poll interval in milliseconds when checking if a credential report is available." default:"5000" env:"POLL_INTERVAL"`
	SlackEmoji         string `long:"slack-emoji" description:"The Slack Emoji associated with the notifications." env:"SLACK_EMOJI" default:":key:"`
	SSMSlackWebhookURL string `long:"ssm-slack-webhook-url" description:"The name of the Slack Webhook Url in Parameter store." required:"false" env:"SSM_SLACK_WEBHOOK_URL"`
	SlackChannel       string `long:"slack-channel" description:"The Slack channel." required:"true" env:"SLACK_CHANNEL"`
}

var options Options
var logger *zap.Logger

func rowToMap(header []string, row []string) map[string]string {
	m := map[string]string{}
	for i, h := range header {
		m[strings.ToLower(h)] = row[i]
	}
	return m
}

func getCredentialReport(iamClient *iam.IAM, tries int, pollInterval int) (*iam.GetCredentialReportOutput, error) {
	if tries <= 0 {
		return &iam.GetCredentialReportOutput{}, errors.New("maxmimum number of tries to get credential report reached")
	}
	report, err := iamClient.GetCredentialReport(&iam.GetCredentialReportInput{})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == iam.ErrCodeCredentialReportNotPresentException || aerr.Code() == iam.ErrCodeCredentialReportExpiredException {
				_, err = iamClient.GenerateCredentialReport(&iam.GenerateCredentialReportInput{})
				if err != nil {
					if aerr, ok = err.(awserr.Error); ok {
						if aerr.Code() == iam.ErrCodeLimitExceededException {
							time.Sleep(time.Duration(pollInterval) * time.Millisecond)
							return getCredentialReport(iamClient, tries-1, pollInterval)
						}
					}
					return &iam.GetCredentialReportOutput{}, err
				}
			} else if aerr.Code() == iam.ErrCodeCredentialReportNotReadyException {
				time.Sleep(time.Duration(pollInterval) * time.Millisecond)
				return getCredentialReport(iamClient, tries-1, pollInterval)
			}
		}
		return &iam.GetCredentialReportOutput{}, err
	}
	return report, nil
}

func sendAlertToSlack(slackWebhookURL string, usersSet stringSet, maxDays float64) error {
	slack := slackhook.New(slackWebhookURL)
	attachment := slackhook.Attachment{
		Title:     "Expired IAM Access Keys",
		Text:      fmt.Sprintf("IAM users with access keys older than %d days", int(maxDays)),
		TitleLink: "https://console.aws.amazon.com/iam/home?region=us-west-2#/users",
		Color:     "warn",
		Footer:    "IAM Keys Check",
		Fields: []slackhook.Field{
			{
				Title: "IAM Users",
				Value: strings.Join(usersSet.toArray(true), ", "),
			},
		},
	}

	if options.DocumentationURL != "" {
		attachment.Fields = append(attachment.Fields, slackhook.Field{
			Title: "Access Key Rotation Instructions",
			Value: options.DocumentationURL,
		})
	}

	message := &slackhook.Message{
		Channel:   options.SlackChannel,
		IconEmoji: options.SlackEmoji,
	}
	message.AddAttachment(&attachment)

	err := slack.Send(message)
	if err != nil {
		return err
	}
	logger.Info("successfully sent slack message", zap.String("slack-channel", options.SlackChannel))
	return nil
}

func triggerCheck() {

	maxDays := float64(options.MaxDays)

	if maxDays == 0 {
		logger.Fatal("days must be greater than 0.")
	}

	sess := session.MustMakeSession(options.Region, options.Profile)
	iamClient := iam.New(sess)

	slackWebhookURL, err := ssm.DecryptValue(sess, options.SSMSlackWebhookURL)
	if err != nil {
		logger.Fatal("failed to decrypt slackWebhookURL", zap.Error(err))
	}

	report, err := getCredentialReport(iamClient, 5, int(options.PollInterval))
	if err != nil {
		logger.Fatal("failed to get credential report", zap.Error(err))
	}

	reader := csv.NewReader(strings.NewReader(string(report.Content)))

	header, err := reader.Read()
	if err != nil {
		if err != io.EOF {
			logger.Fatal("failed to read header from csv", zap.Error(err))
		}
	}

	usersSet := newstringSet()

	for {

		inRow, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				logger.Fatal("failed to read row from csv", zap.Error(err))
			}
		}

		input := rowToMap(header, inRow)

		if input["access_key_1_active"] == "true" {
			accessKey1LastRotated, err := parseTimestamp(input["access_key_1_last_rotated"])
			if err != nil {
				logger.Fatal("failed to parse access_key_1_last_rotated", zap.Error(err))
			}
			if days := report.GeneratedTime.Sub(accessKey1LastRotated).Hours() / 24; days > maxDays {
				usersSet.add(input["user"])
			}
		}

		if !usersSet.has(input["user"]) {
			if input["access_key_2_active"] == "true" {
				accessKey2LastRotated, err := parseTimestamp(input["access_key_2_last_rotated"])
				if err != nil {
					logger.Fatal("failed to parse access_key_2_last_rotated", zap.Error(err))
				}
				if days := report.GeneratedTime.Sub(accessKey2LastRotated).Hours() / 24; days > maxDays {
					usersSet.add(input["user"])
				}
			}
		}

	}

	if usersSet.len() > 0 {
		err = sendAlertToSlack(slackWebhookURL, usersSet, maxDays)
		if err != nil {
			logger.Fatal("failed to send alert to slack", zap.Error(err))
		}
	}

}

func lambdaHandler() {
	lambda.Start(triggerCheck)
}

func main() {

	parser := flag.NewParser(&options, flag.Default)
	_, err := parser.Parse()
	if err != nil {
		log.Fatal(err)
	}

	logger, err = zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}

	if options.Lambda {
		logger.Info("running Lambda handler.")
		lambdaHandler()
	} else {
		triggerCheck()
	}
}
