package main

import (
	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/pkg/amiclean"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	flag "github.com/jessevdk/go-flags"
	"go.uber.org/zap"

	"log"
	"time"
)

// The Options struct describes the command line options available.
type Options struct {
	Delete        bool   `short:"D" long:"delete" env:"DELETE" description:"Actually purge AMIs (runs in dryrun mode by default)."`
	NamePrefix    string `long:"prefix" env:"NAME_PREFIX" description:"Name prefix to filter on (not affected by --invert)."`
	RetentionDays int    `long:"days" default:"30" env:"RETENTION_DAYS" description:"Age of AMI in days before it is a candidate for removal."`
	TagKey        string `long:"tag-key" env:"TAG_KEY" description:"Key of tag to operate on. If you specify a Key, you must also specify a Value."`
	TagValue      string `long:"tag-value" env:"TAG_VALUE" description:"Value of tag to operate on. If you specify a Value, you must also specify a Key."`
	Invert        bool   `short:"i" long:"invert" env:"INVERT" description:"Operate in inverted mode -- only purge AMIs that do NOT match the Tag provided."`
	Unused        bool   `long:"unused" env:"UNUSED" description:"Only purge AMIs for which no running instances were built from."`
	Role          string `long:"sts-role" env:"STS_ROLE" required:"false" description:"The AWS IAM Role name used for cross-account unused AMIs checking."`
	Profile       string `short:"p" long:"profile" env:"AWS_PROFILE" required:"false" description:"The AWS profile to use."`
	Region        string `short:"r" long:"region" env:"AWS_REGION" required:"false" description:"The AWS region to use."`
	Lambda        bool   `long:"lambda" required:"false" env:"LAMBDA" description:"Run as an AWS Lambda function."`
}

var options Options
var logger *zap.Logger

func cleanImages() {
	now := time.Now().UTC()
	// We need to check to make sure that if we have a Tag Key, we also have
	// a Tag Value.
	if (options.TagKey == "") != (options.TagValue == "") {
		logger.Fatal("must specify both a tag Key and tag Value")
	}

	sess := session.MustMakeSession(options.Region, options.Profile)

	a := amiclean.AMIClean{
		NamePrefix:     options.NamePrefix,
		Tag:            &ec2.Tag{Key: aws.String(options.TagKey), Value: aws.String(options.TagValue)},
		Delete:         options.Delete,
		Invert:         options.Invert,
		Unused:         options.Unused,
		Role:           options.Role,
		ExpirationDate: now.AddDate(0, 0, -int(options.RetentionDays)),
		Logger:         logger,
		EC2Client:      session.MakeEC2Client(sess),
		STSClient:      session.MakeSTSClient(sess),
	}

	// Get the list of images that we want to evaluate from AWS.
	availableImages, err := a.GetImages()
	if err != nil {
		logger.Fatal("unable to get list of available images",
			zap.Error(err),
		)
	}

	// For each image in the list, check to see if it matches the criteria.
	for _, image := range availableImages.Images {
		if a.CheckImage(image) {
			// If it matches the criteria, we want to delete it.
			retVal, err := a.PurgeImage(image)
			// If we get an error, we stop the train.
			if err != nil {
				logger.Fatal("Failed to purge image",
					zap.String("ami-id", *image.ImageId),
					zap.String("ami-name", *image.Name),
					zap.String("failure", retVal),
					zap.Error(err),
				)
			}
			// No error, so log success (based on whether we're in
			// delete mode or not).
			if a.Delete {
				logger.Info("Successfully purged image",
					zap.String("ami-id", retVal),
					zap.String("ami-name", *image.Name),
				)
			} else {
				logger.Info("Would have purged image",
					zap.String("ami-id", retVal),
					zap.String("ami-name", *image.Name),
				)
			}
		}
	}

}

func lambdaHandler() {
	lambda.Start(cleanImages)
}

func main() {
	// First, parse out our command line options:
	parser := flag.NewParser(&options, flag.Default)
	_, err := parser.Parse()
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the zap logger:
	logger, err = zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}

	// We need to check to see if we were called as a Lambda function.
	if options.Lambda {
		logger.Info("Running Lambda handler.")
		lambdaHandler()
	} else {
		cleanImages()
	}

}
