package ecsdeploy

import (
	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/service/ecs"
	"go.uber.org/zap"
)

// ECSClusterServiceDeployer defines an ECS Service configuration on a particular ECS Cluster.
type ECSClusterServiceDeployer struct {
	ECSCluster string
	ECSService string
	DryRun     bool
	Logger     *zap.Logger
	ECSClient  *ecs.ECS
}

func stringToStringSlice(originalString string) []string {
	slice := make([]string, 1)
	slice[0] = originalString
	return slice
}

// GetServiceTaskDefiniton returns the primary/active task definition arn for the service/cluster combination
func (e *ECSClusterServiceDeployer) GetServiceTaskDefiniton() (*ecs.TaskDefinition, error) {
	describeServiceInput := &ecs.DescribeServicesInput{
		Cluster:  aws.String(e.ECSCluster),
		Services: aws.StringSlice(stringToStringSlice(e.ECSService)),
	}

	response, err := e.ECSClient.DescribeServices(describeServiceInput)
	taskDefinitionArn := response.Services[0].TaskDefinition

	if err != nil {
		return nil, err
	}

	e.Logger.Info("Found task definition arn.", zap.String("taskDefinitionArn", *taskDefinitionArn))

	describeTaskInput := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: taskDefinitionArn,
	}

	r, err := e.ECSClient.DescribeTaskDefinition(describeTaskInput)

	if err != nil {
		return nil, err
	}

	return r.TaskDefinition, nil
}

// RegisterUpdatedTaskDefinition registers a new task definition based on the existing service definition with updated container images and returns the new taskdefinition arn and errors
func (e *ECSClusterServiceDeployer) RegisterUpdatedTaskDefinition(taskDefinition *ecs.TaskDefinition, containerMap map[string]map[string]string) (*ecs.TaskDefinition, error) {
	// example map map[atlantis:map[image:runatlantis/atlantis:latest]]

	for containerName := range containerMap {
		for idx, containerDefinition := range taskDefinition.ContainerDefinitions {
			// find someone to help me figure this bs out
			containerDefName := *containerDefinition.Name
			if containerDefName == containerName {
				*taskDefinition.ContainerDefinitions[idx].Image = containerMap[containerName]["image"]
			}
		}
	}

	input := &ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions:    taskDefinition.ContainerDefinitions,
		Cpu:                     taskDefinition.Cpu,
		ExecutionRoleArn:        taskDefinition.ExecutionRoleArn,
		Family:                  taskDefinition.Family,
		TaskRoleArn:             taskDefinition.TaskRoleArn,
		NetworkMode:             taskDefinition.NetworkMode,
		Memory:                  taskDefinition.Memory,
		RequiresCompatibilities: taskDefinition.RequiresCompatibilities,
	}

	response, err := e.ECSClient.RegisterTaskDefinition(input)

	if err != nil {
		return nil, err
	}

	taskDefinitionArn := response.TaskDefinition.TaskDefinitionArn

	e.Logger.Info("Created new task definition.", zap.String("taskDefinitionArn", *taskDefinitionArn))
	return response.TaskDefinition, nil

}

// UpdateService updates the Service with the given task definition ARN and returns the ServiceArn
func (e *ECSClusterServiceDeployer) UpdateService(taskDefinitionArn string) (*ecs.Service, error) {
	input := &ecs.UpdateServiceInput{
		ForceNewDeployment: aws.Bool(true),
		Cluster:            aws.String(e.ECSCluster),
		Service:            aws.String(e.ECSService),
		TaskDefinition:     aws.String(taskDefinitionArn),
	}

	response, err := e.ECSClient.UpdateService(input)

	if err != nil {
		return nil, err
	}

	return response.Service, nil
}
