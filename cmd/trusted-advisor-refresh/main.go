package main

import (
	"log"

	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/pkg/tarefresh"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/support"
	flag "github.com/jessevdk/go-flags"
	"go.uber.org/zap"
)

// Options are the command line options
type Options struct {
	Profile string `short:"p" long:"profile" description:"The AWS profile to use." required:"false" env:"AWS_PROFILE"`
	Lambda  bool   `short:"l" long:"lambda" description:"Run as an AWS lambda function." required:"false" env:"LAMBDA"`
	Region  string `long:"region" description:"The AWS region to use." required:"false" env:"AWS_REGION"`
}

var options Options
var logger *zap.Logger

func makeSupportClient(region, profile string) *support.Support {
	sess := session.MustMakeSession(region, profile)
	supportClient := support.New(sess)
	return supportClient
}

func triggerRefresh() {
	// Trusted Advisor only works in us-east-1
	supportClient := makeSupportClient(options.Region, options.Profile)

	tar := tarefresh.TrustedAdvisorRefresh{
		Logger:        logger,
		SupportClient: supportClient,
	}
	err := tar.Refresh()
	if err != nil {
		logger.Fatal("failed to refresh trusted advisor", zap.Error(err))
	}
}

func lambdaHandler() {
	lambda.Start(triggerRefresh)
}

func main() {
	var options Options

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
		triggerRefresh()
	}
}
