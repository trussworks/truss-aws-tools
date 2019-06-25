package main

import (
	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/pkg/packerjanitor"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/ec2"
	flag "github.com/jessevdk/go-flags"
	"go.uber.org/zap"

	"log"
	"time"
)

// Options describes the command line options available.
type Options struct {
	Delete    bool   `short:"D" long:"delete" env:"DELETE" description:"Actually purge AWS resources (runs in dryrun mode by default)."`
	Lambda    bool   `long:"lambda" env:"LAMBDA" required:"false" description:"Run as an AWS Lambda function."`
	TimeLimit int    `short:"t" long:"timelimit" default:"4" env:"TIMELIMIT" description:"Number of hours after which Packer resources should be considered abandoned."`
	Profile   string `short:"p" long:"profile" env:"AWS_PROFILE" required:"false" description:"The AWS profile to use."`
	Region    string `short:"r" long:"region" env:"AWS_REGION" required:"false" description:"The AWS region to use."`
}

var options Options
var logger *zap.Logger

// makeEC2Client establishes our session with AWS.
func makeEC2Client(region, profile string) *ec2.EC2 {
	sess := session.MustMakeSession(region, profile)
	ec2Client := ec2.New(sess)
	return ec2Client
}

// cleanPackerResources is where the work is being done here.
func cleanPackerResources() {
	now := time.Now().UTC()

	p := packerjanitor.PackerClean{
		Delete:         options.Delete,
		ExpirationDate: now.Add(time.Hour * time.Duration(-options.TimeLimit)),
		Logger:         logger,
		EC2Client:      makeEC2Client(options.Region, options.Profile),
	}

	// First, we get the list of instances that fulfills our
	// requirements from EC2.
	packerInstanceList, err := p.GetPackerInstances()
	if err != nil {
		logger.Fatal("unable to get list of Packer instances",
			zap.Error(err),
		)
	}

	// Now, for each instance, we want to purge it and its associated
	// resources. First, let's check to see if the list is empty; if
	// it is, we can just skip the rest.
	if len(packerInstanceList) == 0 {
		logger.Info("No abandoned Packer instances found.")
	} else {
		for _, instance := range packerInstanceList {
			err := p.PurgePackerResource(instance)
			if err != nil {
				logger.Fatal("Failed to purge Packer instance and associated resources",
					zap.String("instance-id", *instance.InstanceId),
					zap.String("keyname", *instance.KeyName),
					zap.String("securitygroup-id", *instance.SecurityGroups[0].GroupId),
					zap.Error(err),
				)
			}
			// If we didn't error out, it worked! Log our
			// success.
			if p.Delete {
				logger.Info("Successfully purged Packer instance and associated resources",
					zap.String("instance-id", *instance.InstanceId),
					zap.String("keyname", *instance.KeyName),
					zap.String("securitygroup-id", *instance.SecurityGroups[0].GroupId),
				)
			} else {
				logger.Info("Would have purged Packer instance and associated resources",
					zap.String("instance-id", *instance.InstanceId),
					zap.String("keyname", *instance.KeyName),
					zap.String("securitygroup-id", *instance.SecurityGroups[0].GroupId),
				)
			}
		}
	}

}

func lambdaHandler() {
	lambda.Start(cleanPackerResources)
}

func main() {
	// First, parse out our command line options:
	parser := flag.NewParser(&options, flag.Default)
	_, err := parser.Parse()
	if err != nil {
		log.Fatalf("could not parse options: %v", err)
	}

	// Initialize the zap logger:
	logger, err = zap.NewProduction()
	if err != nil {
		log.Fatalf("could not initialize zap logger: %v", err)
	}

	// Last thing -- see if we were called as a Lambda function.
	if options.Lambda {
		logger.Info("Running Lambda handler.")
		lambdaHandler()
	} else {
		cleanPackerResources()
	}

}
