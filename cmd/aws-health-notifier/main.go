package main

import (
	"encoding/json"
	"log"

	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/internal/aws/ssm"
	"github.com/trussworks/truss-aws-tools/pkg/awshealth"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	flag "github.com/jessevdk/go-flags"
	"github.com/lytics/slackhook"
	"go.uber.org/zap"
)

// Options are the command line options
type Options struct {
	Region             string `long:"region" description:"The AWS region to use." required:"false" env:"REGION"`
	Profile            string `short:"p" long:"profile" description:"The AWS profile to use." required:"false" env:"AWS_PROFILE"`
	SlackChannel       string `long:"slack-channel" description:"The Slack channel." required:"true" env:"SLACK_CHANNEL"`
	SlackEmoji         string `long:"slack-emoji" description:"The Slack Emoji associated with the notifications." env:"SLACK_EMOJI" default:":boom:"`
	SSMSlackWebhookURL string `long:"ssm-slack-webhook-url" description:"The name of the Slack Webhook Url in Parameter store." required:"false" env:"SSM_SLACK_WEBHOOK_URL"`
}

var options Options
var logger *zap.Logger

func sendNotification(event events.CloudWatchEvent) {
	var health awshealth.Event
	err := json.Unmarshal([]byte(event.Detail), &health)
	if err != nil {
		logger.Error("Unable to unmarshal health event", zap.Error(err))
	}

	eventURL := health.HealthEventURL()
	awsSession := session.MustMakeSession(options.Region, options.Profile)
	slackWebhookURL, err := ssm.DecryptValue(awsSession, options.SSMSlackWebhookURL)
	if err != nil {
		log.Fatal("failed to decrypt slackWebhookURL", zap.Error(err))
	}
	slack := slackhook.New(slackWebhookURL)

	attachment := slackhook.Attachment{
		Title:     "AWS Health Notification",
		TitleLink: awshealth.PersonalHealthDashboardURL,
		Color:     "danger",
		Fields: []slackhook.Field{
			{
				Title: "Service",
				Value: health.Service,
			},
			{
				Title: "Description",
				Value: health.Description[0].Latest,
			},
			{
				Title: "EventTypeCode",
				Value: health.EventTypeCode,
			},
			{
				Title: "Link",
				Value: eventURL,
				Short: false,
			},
		},
	}

	message := &slackhook.Message{
		Channel:   options.SlackChannel,
		IconEmoji: options.SlackEmoji,
	}
	message.AddAttachment(&attachment)

	err = slack.Send(message)
	if err != nil {
		logger.Error("failed to send slack message", zap.Error(err),
			zap.String("slack-channel", options.SlackChannel))
	}
	logger.Info("successfully sent slack message", zap.String("slack-channel", options.SlackChannel))
}

func lambdaHandler() {
	lambda.Start(sendNotification)
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

	logger.Info("Running Lambda handler.")
	lambdaHandler()

}
