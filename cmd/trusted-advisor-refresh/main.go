package main

import (
	"log"
	"os"

	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/pkg/tarefresh"

	"github.com/aws/aws-sdk-go/service/support"
	flag "github.com/jessevdk/go-flags"
	"go.uber.org/zap"
)

// Options are the command line options
type Options struct {
	Profile string `short:"p" long:"profile" description:"The AWS profile to use." required:"false"`
}

// makeSupportClient makes an Support client
func makeSupportClient(region, profile string) *support.Support {
	sess := session.MustMakeSession(region, profile)
	supportClient := support.New(sess)
	return supportClient
}

func main() {
	var options Options

	parser := flag.NewParser(&options, flag.Default)
	_, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	// Trusted Advisor only works in us-east-1
	supportClient := makeSupportClient("us-east-1", options.Profile)
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}

	tar := tarefresh.TrustedAdvisorRefresh{Logger: logger, SupportClient: supportClient}
	err = tar.Refresh()
	if err != nil {
		logger.Fatal("failed to refresh trusted advisor", zap.Error(err))
	}
}
