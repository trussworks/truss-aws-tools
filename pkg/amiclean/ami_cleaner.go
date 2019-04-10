package amiclean

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"

	"strings"
	"time"
)

const (
	// RFC8601 is the date/time format used by AWS.
	RFC8601 = "2006-01-02T15:04:05.000Z"
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
		a.Logger.Info("raw image creation date", zap.String("time", *image.CreationDate))
		//ct := *image.CreationDate
		imageCreationTime, _ := time.Parse(RFC8601, *image.CreationDate)
		a.Logger.Info("parsed image creation date", zap.String("time", imageCreationTime.String()))
		if imageCreationTime.After(a.ExpirationDate) {
			continue
		} else {
			if strings.HasPrefix(a.Branch, "!") {
				branchname := a.Branch[1:]
				for _, tag := range image.Tags {
					if *tag.Key == "Branch" && *tag.Value != branchname {
						a.Logger.Info("selected ami for purging",
							zap.String("ami-id",
								*image.ImageId),
							zap.String("ami-branch-tag",
								*tag.Value),
							zap.String("ami-creation-date",
								imageCreationTime.String()),
						)
						ImagesToPurge =
							append(ImagesToPurge, image)
					}
				}
			} else {
				a.Logger.Info("Entering branch mode", zap.String("branchname", a.Branch))
				for _, tag := range image.Tags {
					if *tag.Key == "Branch" && *tag.Value == a.Branch {
						a.Logger.Info("selected ami for purging",
							zap.String("ami-id",
								*image.ImageId),
							zap.String("ami-branch-tag",
								*tag.Value),
							zap.String("ami-creation-date",
								imageCreationTime.String()),
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
	for _, image := range images {
		// There may be multiple snapshots attached to a single AMI,
		// so we need to build a list and iterate on them.
		var snapshotIds []*string
		for _, blockDevice := range image.BlockDeviceMappings {
			snapshotId := *blockDevice.Ebs.SnapshotId
			snapshotIds = append(snapshotIds, &snapshotId)
		}
		deregisterInput := &ec2.DeregisterImageInput{
			DryRun:  aws.Bool(a.DryRun),
			ImageId: aws.String(*image.ImageId),
		}
		if a.DryRun {
			a.Logger.Info("would deregister ami",
				zap.String("ami-id", *image.ImageId),
			)
		} else {
			a.Logger.Info("deregistering ami",
				zap.String("ami-id", *image.ImageId),
			)
			_, err := a.EC2Client.DeregisterImage(deregisterInput)
			if err != nil {
				return err
			}
		}
		for _, snapshot := range snapshotIds {
			deleteInput := &ec2.DeleteSnapshotInput{
				DryRun:     aws.Bool(a.DryRun),
				SnapshotId: aws.String(*snapshot),
			}
			if a.DryRun {
				a.Logger.Info("would delete snapshot",
					zap.String("snapshot-id", *deleteInput.SnapshotId),
				)
			} else {
				a.Logger.Info("deleting snapshot",
					zap.String("snapshot-id", *deleteInput.SnapshotId),
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
