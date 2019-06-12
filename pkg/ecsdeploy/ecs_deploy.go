package ecsdeploy

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/service/ecs"
	"go.uber.org/zap"
)

// ECSClusterServiceDeployer defines an ECS Service configuration on a particular ECS Cluster.
type ECSClusterServiceDeployer struct {
	ECSCluster string
	ECSService string
	Logger     *zap.Logger
	ECSClient  *ecs.ECS
}

// GetServiceTaskDefinition returns the primary/active task definition arn for the service/cluster combination
func (e *ECSClusterServiceDeployer) GetServiceTaskDefinition() (*ecs.TaskDefinition, error) {
	describeServiceInput := &ecs.DescribeServicesInput{
		Cluster:  aws.String(e.ECSCluster),
		Services: aws.StringSlice([]string{e.ECSService}),
	}

	response, err := e.ECSClient.DescribeServices(describeServiceInput)
	e.Logger.Info("Describe service output", zap.Int("service count", len(response.Services)), zap.Error(err))
	if err != nil {
		return nil, err
	} else if len(response.Services) <= 0 {
		return nil, fmt.Errorf("No services named %s found on cluster %s", e.ECSService, e.ECSCluster)
	}
	taskDefinitionArn := response.Services[0].TaskDefinition

	e.Logger.Info("Found task definition arn", zap.String("taskDefinitionArn", *taskDefinitionArn))

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
	for containerName := range containerMap {
		for idx, containerDefinition := range taskDefinition.ContainerDefinitions {
			if containerName == *containerDefinition.Name {
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
