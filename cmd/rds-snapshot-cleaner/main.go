package main

import (
	"log"
	"time"

	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/pkg/rdsclean"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/rds"
	flag "github.com/jessevdk/go-flags"
	"go.uber.org/zap"
)

// Options are the command line options
type Options struct {
	DBInstanceIdentifier string `long:"db-instance-identifier" description:"The RDS database instance identifier." required:"true" env:"DB_INSTANCE_IDENTIFIER"`
	DryRun               bool   `long:"dry-run" description:"Don't make any changes and log what would have happened." env:"DRY_RUN"`
	Lambda               bool   `long:"lambda" description:"Run as an AWS lambda function." required:"false" env:"LAMBDA"`
	MaxDBSnapshotCount   uint   `long:"max-snapshots" description:"The maximum number of manual snapshots allowed. This takes precedence over -retention-days." default:"0" env:"MAX_DB_SNAPSHOT_COUNT"`
	Profile              string `long:"profile" description:"The AWS profile to use." required:"false" env:"PROFILE"`
	Region               string `long:"region" description:"The AWS region to use." required:"false" env:"REGION"`
	RetentionDays        uint   `long:"retention-days" description:"The maximum retention age in days." default:"30" env:"RETENTION_DAYS"`
}

var options Options
var logger *zap.Logger

func makeRDSClient(region, profile string) *rds.RDS {
	sess := session.MustMakeSession(region, profile)
	rdsClient := rds.New(sess)
	return rdsClient
}

func cleanRDSSnapshots() {
	now := time.Now().UTC()
	r := rdsclean.RDSManualSnapshotClean{
		DBInstanceIdentifier: options.DBInstanceIdentifier,
		DryRun:               options.DryRun,
		ExpirationDate:       now.AddDate(0, 0, -int(options.RetentionDays)),
		Logger:               logger,
		MaxDBSnapshotCount:   options.MaxDBSnapshotCount,
		RDSClient:            makeRDSClient(options.Region, options.Profile),
	}

	manualDBSnapshots, err := r.FindManualDBSnapshots()
	if err != nil {
		logger.Fatal("unable to find manual snapshots",
			zap.Error(err))
	}

	dbSnapshotsToDelete, err := r.FindDBSnapshotsToDelete(manualDBSnapshots)
	if err != nil {
		logger.Fatal("unable to find snapshots to snapshots to delete",
			zap.Error(err))
	}

	err = r.DeleteDBSnapshots(dbSnapshotsToDelete)
	if err != nil {
		logger.Fatal("unable to delete snapshots",
			zap.Error(err))
	}

}

func lambdaHandler() {
	lambda.Start(cleanRDSSnapshots)
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
		cleanRDSSnapshots()
	}

}
