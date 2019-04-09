package amiclean

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws/aws-sdk-go/aws"
	"go.uber.org/zap"

	"strings"
	"time"
)

const (
	// RFC8601 is the date/time format used by AWS.
	RFC8601 = "2006-01-02T15:04:05-07:00"
)

// AMIClean defines parameters for cleaning up AMIs based on the Branch and
// Expiration Date.
type AMIClean struct {
	Branch         string
	DryRun         bool
	ExpirationDate time.Time
	Logger         *zap.Logger
	EC2Client      *ec2.EC2
}

// GetImages gets us all the private AMIs on our account so that they can be
// looked through later. We have to do this here because the AWS API does not
// allow you to search for AMIs by creation date or by *not* having a tag set to
// a certain value, which would speed this up considerably.
func (a *AMIClean) GetImages() (*ec2.DescribeImagesOutput, error) {
	var output *ec2.DescribeImagesOutput

	input := &ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("is-public"),
				Values: []*string{aws.String("false")},
			},
		},
	}

	output, err := a.EC2Client.DescribeImages(input)

	if err != nil {
		return nil, err
	}

	return output, err
}

// FindImagesToPurge looks through the AMIs available and produces a slice of
// ec2.Image objects to put on the chopping block based on the contents of the
// AMIClean struct.
func (a *AMIClean) FindImagesToPurge(output *ec2.DescribeImagesOutput) []*ec2.Image {
	var ImagesToPurge []*ec2.Image
	for _, image := range output.Images {
		ct := *image.CreationDate
		imageCreationTime, _ := time.Parse(RFC8601, ct)
		if imageCreationTime.After(ExpirationDate) {
			continue
		} else {
			if strings.Prefix(a.Branch, "!") {
				branchname := a.Branch[1:]
				for _, tag := range image.Tags {
					if *tag.Key == "Branch" && *tag.Value != branchname {
						a.Logger.Info("selected ami for
						purging",
							zap.String("ami-id",
							image.ImageId),
							zap.String("ami-branch-tag",
							branchname),
							zap.String("ami-creation-date",
							imageCreationTime),
						)
						ImagesToPurge =
							append(ImagesToPurge, image)
					}
				}
			} else {
				for _, tag := range image.Tags {
					if *tag.Key == "Branch" && *tag.Value == a.Branch {
						a.Logger.Info("selected ami for
						purging",
							zap.String("ami-id",
							image.ImageId),
							zap.String("ami-branch-tag",
							a.Branch),
							zap.String("ami-creation-date",
							imageCreationTime),
						)
						ImagesToPurge =
							append(ImagesToPurge, image)
					}
				}
			}
		}
	}
	return ImagesToPurge
}

// GetIdsToProcess takes a slice of ec2.Image objects and pulls out the AMI IDs
// and snapshot IDs so that we can get rid of them later.
func (a *AMIClean) GetIdsToProcess(images []*ec2.Image) ([]string, []string) {
	var amiIds, snapshotIds []string
	for _, image := images {
		amiId := *image.ImageId
		amiIds = append(amiIds, amiId)
		for _, blockDevice := range image.BlockDeviceMappings {
			snapshotId := *blockDevice.Ebs.SnapshotId
			snapshotIds = append(snapshotIds, snapshotId)
		}
	}
	return amiIds, snapshotIds
}

// DeregisterImageList takes a slice of AMI IDs (as strings) and runs the
// deregister operation on each one. This effectively "deletes" the AMI from
// AWS, but we also have to clean up the snapshots.
func (a *AMIClean) DeregisterImageList(imageList []string) error {
	for _, amiId := range imageList {
		deregisterInput := &ec2.DeregisterImageInput{
			DryRun: aws.Bool(a.DryRun),
			ImageId: aws.String(amiId),
		}
		if a.DryRun {
			a.Logger.Info("would deregister ami",
				zap.String("ami-id", amiId)
			)
		} else {
			a.Logger.Info("deregistering ami",
				zap.String("ami-id", amiId)
			)
			output, err := a.EC2Client.DeregisterImage(deregisterInput)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteSnapshotList works mostly the same as DeregisterImageList, only it is
// deleting the snapshots that were linked to the AMIs, taking care of the last
// step to remove these AMIs.
func (a *AMIClean) DeleteSnapshotList(snapshotList []string) error {
	for _, snapshotId := range snapshotList {
		deleteInput := &ec2.DeleteSnapshotInput{
			DryRun: aws.Bool(a.DryRun),
			SnapshotId: aws.String(snapshotId),
		}
		if a.DryRun {
			a.Logger.Info("would delete snapshot",
				zap.String("snapshot-id", snapshotId)
			)
		} else {
			a.Logger.Info("deleting snapshot",
				zap.String("snapshot-id", snapshotId)
			)
			output, err := a.EC2Client.DeleteSnapshot(deleteInput)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
