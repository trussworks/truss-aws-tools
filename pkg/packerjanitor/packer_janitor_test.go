package packerjanitor

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"go.uber.org/zap"
)

// We set up a mock EC2Client so that we can mock API calls for our code.
type mockEC2Client struct {
	ec2iface.EC2API
}

// Setting the time "now" to be midnight on 1 July 2019
var now = time.Date(2019, 7, 1, 0, 0, 0, 0, time.UTC)

var logger, _ = zap.NewProduction()

// This is a Packer instance that is a day old and should be culled in
// our tests.
var packerInstanceOld = &ec2.Instance{
	Tags: []*ec2.Tag{
		{Key: aws.String("Name"), Value: aws.String("Packer Builder")},
	},
	KeyName:    aws.String("packer_1234"),
	LaunchTime: aws.Time(time.Date(2019, 6, 30, 0, 0, 0, 0, time.UTC)),
	InstanceId: aws.String("i-11111111111111111"),
	SecurityGroups: []*ec2.GroupIdentifier{
		{GroupId: aws.String("sg-11111111111111111")},
	},
}

// This is a Packer instance that is a month old and should be culled in
// our tests.
var packerInstanceAncient = &ec2.Instance{
	Tags: []*ec2.Tag{
		{Key: aws.String("Name"), Value: aws.String("Packer Builder")},
	},
	KeyName:    aws.String("packer_1234"),
	LaunchTime: aws.Time(time.Date(2019, 5, 31, 0, 0, 0, 0, time.UTC)),
	InstanceId: aws.String("i-22222222222222222"),
	SecurityGroups: []*ec2.GroupIdentifier{
		{GroupId: aws.String("sg-22222222222222222")},
	},
}

// This is a Packer instance that is only a minute old, so we shouldn't
// be tossing it out in our tests.
var packerInstanceNew = &ec2.Instance{
	Tags: []*ec2.Tag{
		{Key: aws.String("Name"), Value: aws.String("Packer Builder")},
	},
	KeyName:    aws.String("packer_6789"),
	LaunchTime: aws.Time(time.Date(2019, 6, 30, 23, 59, 0, 0, time.UTC)),
	InstanceId: aws.String("i-33333333333333333"),
	SecurityGroups: []*ec2.GroupIdentifier{
		{GroupId: aws.String("sg-33333333333333333")},
	},
}

// This is a helper function for testing whether two slices of instances
// are the same (including order).
func sliceEqual(a, b []*ec2.Instance) bool {
	if (a == nil) != (b == nil) {
		return false
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// Here we're mocking the DescribeInstances call that we'll be using in
// the GetPackerInstances() function test; we are assuming that our
// filtering (based on the tag) will work, so all this does is check that
// the tag in the DescribeInstancesInput filter is set correctly.
func (m *mockEC2Client) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	// There should really be only one filter we're using here, the
	// tag for the Name being "Packer Builder". Making this
	// assumption makes this code much simpler (we don't have to
	// iterate through all the filters).
	if *input.Filters[0].Name == "tag:Name" && *input.Filters[0].Values[0] == "Packer Builder" {
		output := &ec2.DescribeInstancesOutput{
			NextToken: aws.String(""),
			// I'm splitting these up into two reservations
			// to test the looping.
			Reservations: []*ec2.Reservation{
				{Instances: []*ec2.Instance{packerInstanceOld}},
				{Instances: []*ec2.Instance{packerInstanceNew, packerInstanceAncient}},
			},
		}
		return output, nil
	}

	output := &ec2.DescribeInstancesOutput{
		NextToken:    aws.String(""),
		Reservations: []*ec2.Reservation{},
	}

	return output, nil
}

// With the following functions, we're just looking to make sure we're
// using the right inputs and outputs, and that we're not getting errors.
// For a successful test, then, we can just have these be pretty dumb.
func (m *mockEC2Client) TerminateInstances(input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return nil, nil
}

func (m *mockEC2Client) WaitUntilInstanceTerminated(input *ec2.DescribeInstancesInput) error {
	return nil
}

func (m *mockEC2Client) DeleteKeyPair(input *ec2.DeleteKeyPairInput) (*ec2.DeleteKeyPairOutput, error) {
	return nil, nil
}

func (m *mockEC2Client) DeleteSecurityGroup(input *ec2.DeleteSecurityGroupInput) (*ec2.DeleteSecurityGroupOutput, error) {
	return nil, nil
}

// This is a helper function to generate a new PackerClean object;
// we're doing this so that creating a new instance is easy if we
// want to test with various mock EC2 clients.
func testPackerClean(ec2client ec2iface.EC2API) PackerClean {
	output := PackerClean{
		Delete:         true,
		ExpirationDate: now.Add(time.Hour * -4),
		Logger:         logger,
		EC2Client:      ec2client,
	}

	return output
}

// This function exercises the successful functioning of the
// GetPackerInstances function.
func TestGetPackerInstancesSuccess(t *testing.T) {
	// The resultSet we're looking here is all the Packer instances
	// except the new one, which should get filtered out by the
	// ExpirationDate in GetPackerInstances.
	resultSet := []*ec2.Instance{packerInstanceOld, packerInstanceAncient}

	p := testPackerClean(&mockEC2Client{})

	testSet, err := p.GetPackerInstances()

	if err == nil {
		t.Errorf("ERROR: GetPackerInstances threw error during successful test")
	}

	if !sliceEqual(testSet, resultSet) {
		t.Errorf("ERROR: GetPackerInstances failed successful test;\n\texpected: %v,\n\tgot: %v",
			resultSet, testSet,
		)
	}
}

// This function exercises the successful functioning of the
// CleanTerminateInstance function.
func TestCleanTerminateInstanceSuccess(t *testing.T) {
	p := testPackerClean(&mockEC2Client{})
	err := p.CleanTerminateInstance(packerInstanceOld)
	if err != nil {
		t.Errorf("ERROR: CleanTerminateInstance threw error during successful test")
	}
}

// This function exercises the successful functioning of the
// PurgePackerResource function.
func TestPurgePackerResourceSuccess(t *testing.T) {
	p := testPackerClean(&mockEC2Client{})
	err := p.PurgePackerResource(packerInstanceOld)
	if err != nil {
		t.Errorf("ERROR: PurgePackerResource threw error during successful test")
	}
}
