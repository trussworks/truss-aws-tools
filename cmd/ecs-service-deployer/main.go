package main

import (
	"encoding/json"
	"log"

	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"github.com/trussworks/truss-aws-tools/pkg/ecsdeploy"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/ecs"
	flag "github.com/jessevdk/go-flags"
	"go.uber.org/zap"
)

// Options are the command line options
type Options struct {
	ECSCluster string `long:"ecs-cluster-identifier" description:"The ECS cluster identifier." required:"true" env:"ECS_CLUSTER"`
	ECSService string `long:"ecs-service-identifier" description:"The ECS service identifier." required:"true" env:"ECS_SERVICE"`
	Lambda     bool   `long:"lambda" description:"Run as an AWS lambda function." required:"false" env:"LAMBDA"`
	Profile    string `long:"profile" description:"The AWS profile to use." required:"false" env:"PROFILE"`
	Region     string `long:"region" description:"The AWS region to use." required:"false" env:"REGION"`

	Args struct {
		ContainerJSON string
		Rest          []string
	} `positional-args:"yes" required:"yes"`
}

var options Options
var logger *zap.Logger

func makeECSClient(region, profile string) *ecs.ECS {
	sess := session.MustMakeSession(region, profile)
	ecsClient := ecs.New(sess)
	return ecsClient
}

func parseContainerJSON() map[string]map[string]string {
	containerJSONInputBytes := []byte(options.Args.ContainerJSON)
	var parsedJSON map[string]map[string]map[string]string
	err := json.Unmarshal(containerJSONInputBytes, &parsedJSON)
	if err != nil {
		logger.Fatal("Could not unmarshal container json")
	}
	return parsedJSON["containers"]
}

func runECSDeploy() {
	containerMap := parseContainerJSON()

	e := ecsdeploy.ECSClusterServiceDeployer{
		ECSCluster: options.ECSCluster,
		ECSService: options.ECSService,
		Logger:     logger,
		ECSClient:  makeECSClient(options.Region, options.Profile),
	}

	// Get existing configuration for service/cluster configuration
	taskDefinition, err := e.GetServiceTaskDefinition()
	if err != nil {
		logger.Fatal("Unable to get service or task definition", zap.Error(err))
	}

	// Create a new task definition with new container defs
	newTaskDefinition, err := e.RegisterUpdatedTaskDefinition(taskDefinition, containerMap)
	if err != nil {
		logger.Fatal("Unable to register new task definition", zap.Error(err))
	}

	newTaskDefinitionArn := newTaskDefinition.TaskDefinitionArn

	e.Logger.Info("Created new task definition", zap.String("taskDefinitionArn", *newTaskDefinitionArn))

	updatedService, err := e.UpdateService(*newTaskDefinitionArn)

	if err != nil {
		logger.Fatal("Unable to update service to new task definition", zap.Error(err))
	}
	e.Logger.Info("Updated service to use new task definition", zap.String("serviceStatus", *updatedService.Status))

}

func lambdaHandler() {
	lambda.Start(runECSDeploy)
}

func main() {
	// Parse Options
	parser := flag.NewParser(&options, flag.Default)
	_, err := parser.Parse()
	if err != nil {
		log.Fatal(err)
	}

	// Initalize zap logger
	logger, err = zap.NewProduction()
	if err != nil {
		log.Fatalf("Can't initialize zap logger: %v", err)
	}

	if options.Lambda {
		logger.Info("Running Lambda handler")
		lambdaHandler()
	} else {
		runECSDeploy()
	}
}
