package main

import (
	"log"
	"time"

	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/pkg/ebsclean"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/aws/aws-lambda-go/lambda"
	flag "github.com/jessevdk/go-flags"
	"go.uber.org/zap"
)

// Options are the command line options
type Options struct {
	DryRun          bool   `long:"dry-run" env:"DRY_RUN" description:"Don't make any changes and log what would have happened."`
	Lambda          bool   `long:"lambda" env:"LAMBDA" required:"false" description:"Run as an AWS lambda function."`
	Profile         string `long:"profile" env:"PROFILE" required:"false" description:"The AWS profile to use."`
	Region          string `long:"region" env:"REGION" required:"false" description:"The AWS region to use."`
	RetentionDays   uint   `long:"retention-days" env:"RETENTION_DAYS" default:"30" description:"The maximum retention age in days."`
	ExcludeTagKey   string `long:"exclude-tag-key" env:"EXCLUDE_TAG_KEY" required:"false" description:"Key of ebs snapshots tag to exclude from cleanup. If you specify an ExcludeTagKey, you must also specify a ExcludeTagValue."`
	ExcludeTagValue string `long:"exclude-tag-value" env:"EXCLUDE_TAG_VALUE" required:"false" description:"Value of ebs snapshots tag to exclude from cleanup. If you specify a ExcludeTagValue, you must also specify a ExcludeTagKey."`
}

var options Options
var logger *zap.Logger

func cleanEBSSnapshots() {
	now := time.Now().UTC()

	sess := session.MustMakeSession(options.Region, options.Profile)

	e := ebsclean.EBSSnapshotClean{
		DryRun:         options.DryRun,
		ExpirationDate: now.AddDate(0, 0, -int(options.RetentionDays)),
		Logger:         logger,
		EC2Client:      session.MakeEC2Client(sess),
	}

	// Get the list of EBS snapshots that we want to evaluate from AWS.
	availableEBSsnapshots, err := e.GetEBSSnapshots()
	if err != nil {
		logger.Fatal("unable to get list of available ebs snapshots",
			zap.Error(err),
		)
	}

	// For each ebs snapshot in the list, check to see if it matches the criteria.
	for _, snapshot := range availableEBSsnapshots {
		if e.CheckEBSSnapshot(snapshot, &ec2.Tag{
			Key:   aws.String(options.ExcludeTagKey),
			Value: aws.String(options.ExcludeTagValue),
		}) {
			// If it matches the criteria, we want to delete it.
			err := e.DeleteEBSSnapshot(snapshot.SnapshotId)
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case "InvalidSnapshot.InUse":
					e.Logger.Info("Have not deleted snapshot cause found in use",
						zap.String("ebs-snapshot-id", *snapshot.SnapshotId),
					)
					continue
				}
			}
			// If we get an error, we stop the train.
			if err != nil {
				logger.Fatal("Failed to delete ebs snapshot",
					zap.String("ebs-snapshot-id", *snapshot.SnapshotId),
					zap.Error(err),
				)
			}
			// No error, so log success (based on whether we're in
			// dry-run mode or not).
			if e.DryRun {
				logger.Info("Would have deleted ebs snapshot",
					zap.String("ebs-snapshot-id", *snapshot.SnapshotId),
				)
			} else {
				logger.Info("Successfully deleted ebs snapshot",
					zap.String("ebs-snapshot-id", *snapshot.SnapshotId),
				)
			}
		}
	}

}

func lambdaHandler() {
	lambda.Start(cleanEBSSnapshots)
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
		logger.Info("Running Lambda handler.")
		lambdaHandler()
	} else {
		cleanEBSSnapshots()
	}

}
