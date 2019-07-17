package ecsdeploy

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
	"go.uber.org/zap"
)

////////////////////
// Test Data
////////////////////

var goodMatchingContainerMap = map[string]map[string]string{
	"atlantis":            {"image": "updated/imagepath:latest"},
	"nginxbutwithracoons": {"image": "nginxbutwithracoons/imagepath:latest"},
}

var goodNonMatchingContainerMap = map[string]map[string]string{
	"nginx":               {"image": "nginx/imagepath:latest"},
	"nginxbutwithracoons": {"image": "nginxbutwithracoons/imagepath:latest"},
}

var emptyContainerMap = map[string]map[string]string{}

var goodContainerDef = &ecs.ContainerDefinition{
	Name:                   aws.String("atlantis"),
	Image:                  aws.String("runatlantis/atlantis:latest"),
	Cpu:                    aws.Int64(256),
	Memory:                 aws.Int64(512),
	MemoryReservation:      aws.Int64(128),
	PortMappings:           nil,
	Essential:              nil,
	Environment:            nil,
	MountPoints:            nil,
	VolumesFrom:            nil,
	Secrets:                nil,
	ReadonlyRootFilesystem: aws.Bool(false),
	LogConfiguration:       nil,
}

var goodTaskDefinition = &ecs.TaskDefinition{
	ContainerDefinitions:    []*ecs.ContainerDefinition{goodContainerDef},
	Family:                  aws.String("atlantis"),
	TaskRoleArn:             aws.String("arn:aws:iam::accountID:role/atlantis-ecs_task_execution"),
	ExecutionRoleArn:        aws.String("arn:aws:iam::accountID:role/atlantis-ecs_task_execution"),
	NetworkMode:             aws.String("awsvpc"),
	Revision:                aws.Int64(000),
	Status:                  aws.String("ACTIVE"),
	RequiresCompatibilities: []*string{aws.String("FARGATE")},
	Cpu:                     aws.String("256"),
	Memory:                  aws.String("512"),
}

var goodTaskDefResponse = &ecs.RegisterTaskDefinitionOutput{
	TaskDefinition: goodTaskDefinition,
}

////////////////////
// ECS Client Mock
////////////////////
type mockECSClient struct {
	ecsiface.ECSAPI
}

func (m *mockECSClient) DescribeServices(input *ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error) {
	//do the mocking
	return nil, nil
}

func (m *mockECSClient) DescribeTaskDefinition(input *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error) {
	//do the mocking
	return nil, nil
}

// You must mock this one for the thing you NEED to test
func (m *mockECSClient) RegisterTaskDefinition(input *ecs.RegisterTaskDefinitionInput) (*ecs.RegisterTaskDefinitionOutput, error) {
	var taskDefinition = &ecs.TaskDefinition{
		ContainerDefinitions: input.ContainerDefinitions,
		Family:               input.Family,
		TaskRoleArn:          input.TaskRoleArn,
		ExecutionRoleArn:     input.TaskRoleArn,
		NetworkMode:          input.NetworkMode,
		// technically revision should be different this just makes it easier to test
		Revision:                aws.Int64(000),
		Status:                  aws.String("ACTIVE"),
		RequiresCompatibilities: input.RequiresCompatibilities,
		Cpu:                     input.Cpu,
		Memory:                  input.Memory,
	}

	output := &ecs.RegisterTaskDefinitionOutput{
		TaskDefinition: taskDefinition,
	}
	return output, nil
}

func (m *mockECSClient) UpdateService(input *ecs.UpdateServiceInput) (*ecs.UpdateServiceOutput, error) {
	//do the mocking
	return nil, nil
}

/////////////////
// Tests
/////////////////
var logger, _ = zap.NewProduction()
var mockClient = &mockECSClient{}

func TestRegisterUpdatedTaskDefinition(t *testing.T) {
	// Setup for the test
	e := ECSClusterServiceDeployer{
		ECSCluster: "test",
		ECSService: "atlantis",
		Logger:     logger,
		ECSClient:  mockClient,
	}

	// Empty Container map
	fmt.Println("Test empty container map")
	fmt.Println("+++++++++++++++++++")
	taskDefinition, err := e.RegisterUpdatedTaskDefinition(goodTaskDefinition, emptyContainerMap)
	//	if !reflect.DeepEqual(taskDefinition, goodTaskDefinition) {
	//		t.Errorf("ERROR: The task definition changed.")
	//		fmt.Println(taskDefinition)
	//		fmt.Println(goodTaskDefinition)
	//		fmt.Println(err)
	//	}
	fmt.Println("did this mutate?")
	fmt.Println(taskDefinition)
	fmt.Println(err)

	// Best case
	fmt.Println("Test normal case")
	fmt.Println("+++++++++++++++++++")
	taskDefinition, err = e.RegisterUpdatedTaskDefinition(goodTaskDefinition, goodMatchingContainerMap)
	fmt.Println(taskDefinition)
	fmt.Println(goodTaskDefinition)
	fmt.Println(err)
	if !reflect.DeepEqual(taskDefinition, goodTaskDefinition) {
		t.Errorf("ERROR: The task definition changed.")
	}
	fmt.Println("did this mutate?")
	fmt.Println(goodTaskDefinition)
	// Okay case
	// taskDefinition, err = e.RegisterUpdatedTaskDefinition(goodTaskDefinition, goodNonMatchingContainerMap)
	// fmt.Println(taskDefinition)
	// fmt.Println(goodTaskDefinition)
	// fmt.Println(err)

}
