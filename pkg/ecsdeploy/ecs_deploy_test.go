package ecsdeploy

import (
	"errors"
	// "fmt"
	"testing"

	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/ecs/ecsiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

////////////////////
// Shared mocks
////////////////////
type ecsMock struct {
	mock.Mock
	ecsiface.ECSAPI
}

func (m *ecsMock) UpdateService(upi *ecs.UpdateServiceInput) (*ecs.UpdateServiceOutput, error) {
	args := m.Called(upi)
	return args.Get(0).(*ecs.UpdateServiceOutput), args.Error(1)
}

func (m *ecsMock) DescribeServices(input *ecs.DescribeServicesInput) (*ecs.DescribeServicesOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*ecs.DescribeServicesOutput), args.Error(1)
}

func (m *ecsMock) DescribeTaskDefinition(input *ecs.DescribeTaskDefinitionInput) (*ecs.DescribeTaskDefinitionOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*ecs.DescribeTaskDefinitionOutput), args.Error(1)
}

func (m *ecsMock) RegisterTaskDefinition(input *ecs.RegisterTaskDefinitionInput) (*ecs.RegisterTaskDefinitionOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*ecs.RegisterTaskDefinitionOutput), args.Error(1)
}

////////////////////
// Testing Func UpdateService
////////////////////

type GetTaskDefinitionSuite struct {
	suite.Suite
	ecsMockClient *ecsMock
	e             ECSClusterServiceDeployer
	taskDef       string
}

func TestGetTaskDefinitionSuite(t *testing.T) {
	suite.Run(t, new(GetTaskDefinitionSuite))
}

func (suite *GetTaskDefinitionSuite) SetupTest() {
	var logger *zap.Logger // not for sure what to do about zap
	logger, _ = zap.NewDevelopment()
	suite.ecsMockClient = new(ecsMock)
	suite.e = ECSClusterServiceDeployer{
		Logger:     logger,
		ECSCluster: "mycluster",
		ECSService: "myservice",
		ECSClient:  suite.ecsMockClient,
	}
	suite.taskDef = "thisismytaskdef"
}

// TestGetServiceTaskDefinition testing normal operation
func (suite *GetTaskDefinitionSuite) TestGetServiceTaskDefinition() {
	serviceOutput := ecs.DescribeServicesOutput{
		Services: []*ecs.Service{
			&ecs.Service{
				TaskDefinition: &suite.taskDef,
			}},
	}

	taskDefOutput := ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &ecs.TaskDefinition{
			TaskDefinitionArn: &suite.taskDef,
		},
	}

	suite.ecsMockClient.On("DescribeServices", mock.Anything).Once().Return(&serviceOutput, nil)
	suite.ecsMockClient.On("DescribeTaskDefinition", mock.Anything).Once().Return(&taskDefOutput, nil)

	resp, err := suite.e.GetServiceTaskDefinition()
	assert.Equal(suite.T(), resp.TaskDefinitionArn, &suite.taskDef)
	assert.Nil(suite.T(), err)
	suite.ecsMockClient.AssertExpectations(suite.T())
}

// TestGetServiceTaskDefinitionServicesFailed test when the DescribeServices call fails
func (suite *GetTaskDefinitionSuite) TestGetServiceTaskDefinitionServicesFailed() {
	output := ecs.DescribeServicesOutput{}
	suite.ecsMockClient.On("DescribeServices", mock.Anything).Once().Return(&output, errors.New("poof AWS died"))

	_, err := suite.e.GetServiceTaskDefinition()
	assert.Error(suite.T(), err)
	suite.ecsMockClient.AssertExpectations(suite.T())
}

// TestGetServiceTaskDefinitionNoMatchingServices test when service serach return no matching services
func (suite *GetTaskDefinitionSuite) TestGetServiceTaskDefinitionNoMatchingServices() {
	output := ecs.DescribeServicesOutput{
		Services: make([]*ecs.Service, 0),
	}
	suite.ecsMockClient.On("DescribeServices", mock.Anything).Once().Return(&output, nil)

	_, err := suite.e.GetServiceTaskDefinition()
	assert.Error(suite.T(), err)
	suite.ecsMockClient.AssertExpectations(suite.T())
}

// TestGetServiceTaskDefinitionDescribeTaskFails test when DescribeTaskDefinition fails throwing error
func (suite *GetTaskDefinitionSuite) TestGetServiceTaskDefinitionDescribeTaskFails() {
	serviceOutput := ecs.DescribeServicesOutput{
		Services: []*ecs.Service{
			&ecs.Service{
				TaskDefinition: &suite.taskDef,
			}},
	}

	taskDefOutput := ecs.DescribeTaskDefinitionOutput{
		TaskDefinition: &ecs.TaskDefinition{
			TaskDefinitionArn: &suite.taskDef,
		},
	}

	suite.ecsMockClient.On("DescribeServices", mock.Anything).Once().Return(&serviceOutput, nil)
	suite.ecsMockClient.On("DescribeTaskDefinition", mock.Anything).Once().Return(&taskDefOutput, errors.New("aws failed"))

	_, err := suite.e.GetServiceTaskDefinition()
	assert.Error(suite.T(), err)
	suite.ecsMockClient.AssertExpectations(suite.T())
}

////////////////////
// Testing Func UpdateService
////////////////////

type UpdateServiceSuite struct {
	suite.Suite
	ecsMockClient *ecsMock
	e             ECSClusterServiceDeployer
}

func (suite *UpdateServiceSuite) SetupTest() {
	var logger *zap.Logger // not for sure what to do about zap
	logger, _ = zap.NewDevelopment()
	suite.ecsMockClient = new(ecsMock)
	suite.e = ECSClusterServiceDeployer{
		Logger:     logger,
		ECSCluster: "mycluster",
		ECSService: "myservice",
		ECSClient:  suite.ecsMockClient,
	}
}

func TestUpdateServiceSuite(t *testing.T) {
	suite.Run(t, new(UpdateServiceSuite))
}

// TestUpdateService tests the case where the ECS API are a success
func (suite *UpdateServiceSuite) TestUpdateService() {
	service := new(ecs.Service)
	output := ecs.UpdateServiceOutput{
		Service: service,
	}
	suite.ecsMockClient.On("UpdateService", mock.Anything).Return(&output, nil)

	resp, err := suite.e.UpdateService("here is my taskDefin")
	assert.Equal(suite.T(), resp, service)
	assert.Nil(suite.T(), err)

	suite.ecsMockClient.AssertExpectations(suite.T())
}

// TestUpdateServiceError tests the case where the ECS API call throws an error
func (suite *UpdateServiceSuite) TestUpdateServiceError() {
	output := ecs.UpdateServiceOutput{
		Service: nil,
	}
	suite.ecsMockClient.On("UpdateService", mock.Anything).Return(&output, errors.New("poof AWS died"))

	resp, err := suite.e.UpdateService("here is my taskDefin")
	assert.Nil(suite.T(), resp)
	assert.Error(suite.T(), err)

	suite.ecsMockClient.AssertExpectations(suite.T())
}

////////////////////
// Testing RegisterUpdatedTaskDefinition
////////////////////

type RegisterUpdatedTaskDefinitionSuite struct {
	suite.Suite
	ecsMockClient *ecsMock
	e             ECSClusterServiceDeployer
	containerMap  map[string]map[string]string
}

func (suite *RegisterUpdatedTaskDefinitionSuite) SetupTest() {
	var logger *zap.Logger // not for sure what to do about zap
	logger, _ = zap.NewDevelopment()
	suite.ecsMockClient = new(ecsMock)
	suite.e = ECSClusterServiceDeployer{
		Logger:     logger,
		ECSCluster: "mycluster",
		ECSService: "myservice",
		ECSClient:  suite.ecsMockClient,
	}
	suite.containerMap = map[string]map[string]string{
		"atlantis":            {"image": "updated/imagepath:latest"},
		"nginxbutwithracoons": {"image": "nginxbutwithracoons/imagepath:latest"},
	}
}

func TestRegisterUpdatedTaskDefinition(t *testing.T) {
	suite.Run(t, new(RegisterUpdatedTaskDefinitionSuite))
}

// TestUpdateServiceError tests the case where the ECS API call throws an error
func (suite *RegisterUpdatedTaskDefinitionSuite) TestRegisterUpdatedTaskDefinition() {
	output := ecs.RegisterTaskDefinitionOutput{}

	// Ensure the search works correctly
	suite.ecsMockClient.On("RegisterTaskDefinition",
		mock.MatchedBy(
			func(input *ecs.RegisterTaskDefinitionInput) bool {
				return len(input.ContainerDefinitions) == 2
			},
		),
	).Return(&output, nil)

	taskDef := ecs.TaskDefinition{
		ContainerDefinitions: make([]*ecs.ContainerDefinition, 0),
	}
	for container := range suite.containerMap {
		c := ecs.ContainerDefinition{}
		c.SetName(container)
		taskDef.ContainerDefinitions = append(taskDef.ContainerDefinitions, &c)
	}

	resp, err := suite.e.RegisterUpdatedTaskDefinition(&taskDef, suite.containerMap)
	assert.Nil(suite.T(), resp)
	assert.Nil(suite.T(), err)

	suite.ecsMockClient.AssertExpectations(suite.T())
}

// TestRegisterUpdatedTaskDefinitionBadContainers test mismatched input
func (suite *RegisterUpdatedTaskDefinitionSuite) TestRegisterUpdatedTaskDefinitionBadContainers() {
	output := ecs.RegisterTaskDefinitionOutput{}

	// Ensure the search works correctly
	suite.ecsMockClient.On("RegisterTaskDefinition",
		mock.MatchedBy(
			func(input *ecs.RegisterTaskDefinitionInput) bool {
				return len(input.ContainerDefinitions) == 2
			},
		),
	).Return(&output, nil)

	taskDef := ecs.TaskDefinition{
		ContainerDefinitions: make([]*ecs.ContainerDefinition, 0),
	}
	for container := range suite.containerMap {
		c := ecs.ContainerDefinition{}
		c.SetName(container)
		taskDef.ContainerDefinitions = append(taskDef.ContainerDefinitions, &c)
	}

	taskDef.ContainerDefinitions[0].SetName("blah")

	resp, err := suite.e.RegisterUpdatedTaskDefinition(&taskDef, suite.containerMap)
	assert.Nil(suite.T(), resp)
	assert.Nil(suite.T(), err)

	suite.ecsMockClient.AssertExpectations(suite.T())
}

// TestRegisterUpdatedTaskDefinitionFailedAWSCall testing failed aws call
func (suite *RegisterUpdatedTaskDefinitionSuite) TestRegisterUpdatedTaskDefinitionFailedAWSCall() {
	output := ecs.RegisterTaskDefinitionOutput{}

	suite.ecsMockClient.On("RegisterTaskDefinition", mock.Anything).Return(&output, errors.New("AWS went poof"))

	taskDef := ecs.TaskDefinition{}

	_, err := suite.e.RegisterUpdatedTaskDefinition(&taskDef, suite.containerMap)
	assert.Error(suite.T(), err)

	suite.ecsMockClient.AssertExpectations(suite.T())
}
