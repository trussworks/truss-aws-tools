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

// PurgeImages takes a slice of ec2.Image objects and processes them in turn,
// deregistering their AMI ID and then deleting their snapshot IDs. We want to
// do this image by image because if we have an error, we don't want a ton of
// orphaned snapshots lying around.
func (a *AMIClean) PurgeImages(images []*ec2.Image) error {
	for _, image := images {
		amiId := *image.ImageId
		// There may be multiple snapshots attached to a single AMI,
		// so we need to build a list and iterate on them.
		var snapshotIds []*string
		for _, blockDevice := range image.BlockDeviceMappings {
			snapshotId := *blockDevice.Ebs.SnapshotId
			snapshotIds = append(snapshotIds, snapshotId)
		}
		deregisterInput := &ec2.DeregisterImageInput{
			DryRun: aws.Bool(a.Dryrun),
			ImageId: aws.String(amiId),
		}
		if a.DryRun {
			a.Logger.Info("would deregister ami",
				zap.String("ami-id", amiId),
			)
		} else {
			a.Logger.Info("deregistering ami",
				zap.String("ami-id", amiId),
			)
			_, err := a.EC2Client.DeregisterImage(deregister(input)
			if err != nil {
				return err
			}
		}
		for _, snapshot := range snapshotIds {
			deleteInput := &ec2.DeleteSnapshotInput{
				DryRun: aws.Bool(a.DryRun),
				SnapshotId: aws.String(snapshot),
			}
			if a.DryRun {
				a.Logger.Info("would delete snapshot",
					zap.String("snapshot-id", snapshotId),
				)
			} else {
				a.Logger.Info("deleting snapshot",
					zap.String("snapshot-id", snapshotId),
				)
				_, err := a.EC2Client.DeleteSnapshot(deleteInput)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
