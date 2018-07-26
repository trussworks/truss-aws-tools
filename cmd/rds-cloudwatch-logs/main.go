package main

import (
	"log"
	"time"

	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/pkg/rdscwlogs"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/rds"
	flag "github.com/jessevdk/go-flags"
	"go.uber.org/zap"
)

// Options are the command line options
type Options struct {
	CloudWatchLogsGroup  string `long:"cloudwatch-logs-group" description:"The CloudWatch Log group name." required:"true" env:"CLOUDWATCH_LOGS_GROUP"`
	DBInstanceIdentifier string `long:"db-instance-identifier" description:"The RDS database instance identifier." required:"true" env:"DB_INSTANCE_IDENTIFIER"`
	Lambda               bool   `long:"lambda" description:"Run as an AWS lambda function." required:"false" env:"LAMBDA"`
	Profile              string `long:"profile" description:"The AWS profile to use." required:"false" env:"PROFILE"`
	Region               string `long:"region" description:"The AWS region to use." required:"false" env:"REGION"`
	StartTime            string `long:"start-time" description:"The log file start time." required:"true" choice:"1h" choice:"1d" env:"START_TIME"`
}

var options Options
var logger *zap.Logger

func makeCloudWatchLogsClient(region, profile string) *cloudwatchlogs.CloudWatchLogs {
	sess := session.MustMakeSession(region, profile)
	cloudWatchLogsClient := cloudwatchlogs.New(sess)
	return cloudWatchLogsClient
}

func makeRDSClient(region, profile string) *rds.RDS {
	sess := session.MustMakeSession(region, profile)
	rdsClient := rds.New(sess)
	return rdsClient
}

func sendLogs() {
	r := rdscwlogs.RDSCloudWatchLogs{
		DBInstanceIdentifier: options.DBInstanceIdentifier,
		CloudWatchLogsClient: makeCloudWatchLogsClient(options.Region, options.Profile),
		CloudWatchLogsGroup:  options.CloudWatchLogsGroup,
		Logger:               logger,
		RDSClient:            makeRDSClient(options.Region, options.Profile),
	}

	var since int64
	if options.StartTime == "1h" {
		since = time.Now().Add(-1*time.Hour).Unix() * 1000
	} else if options.StartTime == "1d" {
		since = time.Now().Add(-24*time.Hour).Unix() * 1000
	}

	mostRecentDBLogFile, err := r.GetMostRecentLogFile()
	if err != nil {
		logger.Fatal("unable to find most recent rds log file",
			zap.Error(err))
	}

	dbLogFiles, err := r.GetLogFilesSince(since)
	if err != nil {
		logger.Fatal("unable to get rds log files", zap.Error(err))
	}
	for i := range dbLogFiles {
		dbLogFile := dbLogFiles[i]
		if *dbLogFile.LogFileName == *mostRecentDBLogFile.LogFileName {
			logger.Info("skipping most recent db log file",
				zap.String("db_log_file_name", *mostRecentDBLogFile.LogFileName))
			continue
		}
		err = r.SendRDSLogFile(*dbLogFile.LogFileName)
		if err != nil {
			logger.Error("unable to send rds log file to cloudWatch logs",
				zap.Error(err))
		}
	}
}

func lambdaHandler() {
	lambda.Start(sendLogs)
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
		sendLogs()
	}

}
