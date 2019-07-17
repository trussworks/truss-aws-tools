package ecsdeploy

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
	"go.uber.org/zap"
)

// ECSClusterServiceDeployer defines an ECS Service configuration on a particular ECS Cluster.
type ECSClusterServiceDeployer struct {
	ECSCluster string
	ECSService string
	Logger     *zap.Logger
	ECSClient  ecsiface.ECSAPI
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

func copyContainerDef(source *ecs.ContainerDefinition) (dest *ecs.ContainerDefinition) {
	//newImage := *source.Image
	dest = &ecs.ContainerDefinition{
		Command:               source.Command,
		Cpu:                   source.Cpu,
		DisableNetworking:     source.DisableNetworking,
		DnsSearchDomains:      source.DnsSearchDomains,
		DnsServers:            source.DnsServers,
		DockerLabels:          source.DockerLabels,
		DockerSecurityOptions: source.DockerSecurityOptions,
		EntryPoint:            source.EntryPoint,
		Environment:           source.Environment,
		Essential:             source.Essential,
		ExtraHosts:            source.ExtraHosts,
		HealthCheck:           source.HealthCheck,
		Hostname:              source.Hostname,
		//Image:                  &newImage,
		Image:                  source.Image,
		Interactive:            source.Interactive,
		Links:                  source.Links,
		LinuxParameters:        source.LinuxParameters,
		LogConfiguration:       source.LogConfiguration,
		Memory:                 source.Memory,
		MemoryReservation:      source.MemoryReservation,
		MountPoints:            source.MountPoints,
		Name:                   source.Name,
		PortMappings:           source.PortMappings,
		Privileged:             source.Privileged,
		PseudoTerminal:         source.PseudoTerminal,
		ReadonlyRootFilesystem: source.ReadonlyRootFilesystem,
		RepositoryCredentials:  source.RepositoryCredentials,
		Secrets:                source.Secrets,
		SystemControls:         source.SystemControls,
		Ulimits:                source.Ulimits,
		User:                   source.User,
		VolumesFrom:            source.VolumesFrom,
		WorkingDirectory:       source.WorkingDirectory,
	}
	return dest
}

// RegisterUpdatedTaskDefinition registers a new task definition based on the existing service definition with updated container images and returns the new taskdefinition arn and errors
func (e *ECSClusterServiceDeployer) RegisterUpdatedTaskDefinition(taskDefinition *ecs.TaskDefinition, containerMap map[string]map[string]string) (*ecs.TaskDefinition, error) {
	newContainerDefs := make([]*ecs.ContainerDefinition, len(taskDefinition.ContainerDefinitions))
	for containerName := range containerMap {
		for idx, containerDefinition := range taskDefinition.ContainerDefinitions {
			newContainerDefs[idx] = copyContainerDef(containerDefinition)
			//newContainerDefs[idx] = containerDefinition
			fmt.Println("before")
			fmt.Println("=================")
			fmt.Println("Original")
			fmt.Println(containerDefinition.Image)
			fmt.Println(&containerDefinition.Image)
			fmt.Println("New")
			fmt.Println(newContainerDefs[idx].Image)
			fmt.Println(&newContainerDefs[idx].Image)
			if containerName == *containerDefinition.Name {
				fmt.Println("fuck")
				icanteven := containerMap[containerName]["image"]
				fmt.Println(&icanteven)
				newContainerDefs[idx].Image = &icanteven
				//*newContainerDefs[idx].Image = containerMap[containerName]["image"]
				//*taskDefinition.ContainerDefinitions[idx].Image = containerMap[containerName]["image"]
			}
			fmt.Println("after")
			fmt.Println("=================")
			fmt.Println("Original")
			fmt.Println(containerDefinition.Image)
			fmt.Println(&containerDefinition.Image)
			fmt.Println("New")
			fmt.Println(newContainerDefs[idx].Image)
			fmt.Println(&newContainerDefs[idx].Image)
			fmt.Println("=================")
		}
	}
	//	fmt.Println(taskDefinition.ContainerDefinitions)
	//	fmt.Println(newContainerDefs)
	input := &ecs.RegisterTaskDefinitionInput{
		ContainerDefinitions:    newContainerDefs,
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
