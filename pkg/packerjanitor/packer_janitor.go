package packerjanitor

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"go.uber.org/zap"

	"time"
)

const (
	// RFC8601 is the date/time format used by AWS.
	RFC8601 = "2006-01-02T15:04:05.000Z"
	// DryRun is the type of error thrown by AWS when a task fails
	// because it was run with the DryRun option but would have
	// otherwise succeeded.
	DryRun = "DryRunOperation"
)

// PackerClean is a generic struct used for the various functions.
type PackerClean struct {
	Delete         bool
	ExpirationDate time.Time
	Logger         *zap.Logger
	EC2Client      ec2iface.EC2API
}

// GetPackerInstances -- find all running instances that are Packer
// builds older than X and returns them in a list
func (p *PackerClean) GetPackerInstances() ([]*ec2.Instance, error) {
	var output *ec2.DescribeInstancesOutput
	// Instances Packer starts all have the Name tag "Packer Builder".
	packerFilter := &ec2.Filter{
		Name:   aws.String("tag:Name"),
		Values: []*string{aws.String("Packer Builder")},
	}
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{packerFilter},
	}
	output, err := p.EC2Client.DescribeInstances(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				p.Logger.Error("Encountered AWS error attempting to get instance list",
					zap.Error(aerr),
				)
			}
		} else {
			p.Logger.Error("Error while attempting to get instance list",
				zap.Error(err),
			)
			return nil, err
		}
	}

	// The output gives us reservations; we need to get the actual
	// instances out of them, and look to make sure they are older
	// than the time we're looking for.
	var instanceList = []*ec2.Instance{}

	for _, reservation := range output.Reservations {
		for _, instance := range reservation.Instances {
			// We need to check if the instance is older
			// than our expiration, because we can't do
			// that comparison in a filter above. :/
			instanceLaunchTime := *instance.LaunchTime
			if instanceLaunchTime.Before(p.ExpirationDate) {
				instanceList = append(instanceList, instance)
			}
		}
	}

	return instanceList, nil

}

// CleanTerminateInstance -- Terminates an instance and waits until it is
// gone before returning.
func (p *PackerClean) CleanTerminateInstance(instance *ec2.Instance) error {
	terminateInput := &ec2.TerminateInstancesInput{
		DryRun:      aws.Bool(!p.Delete),
		InstanceIds: []*string{instance.InstanceId},
	}
	_, err := p.EC2Client.TerminateInstances(terminateInput)
	if err != nil {
		// Check to see if this was an AWS error
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			// If this was a dryrun operation, we want to
			// log that we would have succeeded and keep
			// going.
			case DryRun:
				p.Logger.Info("Would have terminated instance",
					zap.String("instance-id", *instance.InstanceId),
					zap.Error(aerr),
				)
			default:
				p.Logger.Error("Encountered AWS Error attempting to terminate instance",
					zap.String("instance-id", *instance.InstanceId),
					zap.Error(aerr),
				)
				return aerr
			}
		} else {
			p.Logger.Error("Error while attempting to terminate instance",
				zap.String("instance-id", *instance.InstanceId),
				zap.Error(err),
			)
			return err
		}
	}

	// If we were able to terminate the instance, wait for it to be
	// terminated. If we're in a dry run, just return right away.
	if !p.Delete {
		return nil
	}

	describeInput := &ec2.DescribeInstancesInput{
		DryRun: aws.Bool(!p.Delete),
		Filters: []*ec2.Filter{{
			Name:   aws.String("instance-id"),
			Values: []*string{instance.InstanceId},
		},
		},
	}
	err = p.EC2Client.WaitUntilInstanceTerminated(describeInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				p.Logger.Error("Encountered AWS error waiting for instance to terminate",
					zap.String("instance-id", *instance.InstanceId),
					zap.Error(aerr),
				)
				return aerr
			}
		} else {
			p.Logger.Error("Error while waiting for instance to terminate",
				zap.String("instance-id", *instance.InstanceId),
				zap.Error(err),
			)
			return err
		}
	}

	// Once the instance has terminated, we can return a success.
	return nil

}

// PurgePackerResource -- takes an instance, collects the key and SG
// for it, terminates the instance, waits until it is dead, and then
// deletes the key pair and security group.
func (p *PackerClean) PurgePackerResource(instance *ec2.Instance) error {
	// First, we need to terminate the instance and wait for it
	// to die; we can't delete security groups if an instance is
	// still running with it.
	err := p.CleanTerminateInstance(instance)
	if err != nil {
		p.Logger.Error("Failed to terminate instance",
			zap.String("instance-id", *instance.InstanceId),
			zap.Error(err),
		)
		return err
	}

	// Now that the instance is terminated, let's clean up the
	// keypair.
	deleteKeyInput := &ec2.DeleteKeyPairInput{
		DryRun:  aws.Bool(!p.Delete),
		KeyName: instance.KeyName,
	}
	_, err = p.EC2Client.DeleteKeyPair(deleteKeyInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			// If we were doing this as a dry run, just log
			// that we would have deleted it and continue.
			case DryRun:
				p.Logger.Info("Would have deleted keypair",
					zap.String("keypair", *instance.KeyName),
					zap.Error(aerr),
				)
			default:
				p.Logger.Error("Encountered AWS error while deleting keypair",
					zap.String("keypair", *instance.KeyName),
					zap.Error(aerr),
				)
				return aerr
			}
		} else {
			p.Logger.Error("Error while attempting to delete keypair",
				zap.String("keypair", *instance.KeyName),
				zap.Error(err),
			)
			return err
		}
	}

	// We should also clean up the security group used by the
	// instance.
	deleteSGInput := &ec2.DeleteSecurityGroupInput{
		DryRun: aws.Bool(!p.Delete),
		// There should only be a single security group for a
		// Packer instance, so we can get the first one from
		// this list.
		GroupId: instance.SecurityGroups[0].GroupId,
	}
	_, err = p.EC2Client.DeleteSecurityGroup(deleteSGInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			// Same thing here as the rest; if we are in
			// a dryrun, handwave our success.
			case DryRun:
				p.Logger.Info("Would have deleted security group",
					zap.String("security-group", *instance.SecurityGroups[0].GroupId),
					zap.Error(aerr),
				)
			default:
				p.Logger.Error("Encountered AWS error while deleting security group",
					zap.String("security-group", *instance.SecurityGroups[0].GroupId),
					zap.Error(aerr),
				)
				return aerr
			}
		} else {
			p.Logger.Error("Error attempting to delete security group",
				zap.String("security-group", *instance.SecurityGroups[0].GroupId),
				zap.Error(err),
			)
			return err
		}
	}

	return nil

}
