package main

import (
	"log"

	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/pkg/tarefresh"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/support"
	"go.uber.org/zap"
)

// makeSupportClient makes an Support client
func makeSupportClient(region, profile string) *support.Support {
	sess := session.MustMakeSession(region, profile)
	supportClient := support.New(sess)
	return supportClient
}

func triggerRefresh() {
	// Trusted Advisor only works in us-east-1 and passing a profile is not necessary
	supportClient := makeSupportClient("us-east-1", "")
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

func main() {
	lambda.Start(triggerRefresh)
}
