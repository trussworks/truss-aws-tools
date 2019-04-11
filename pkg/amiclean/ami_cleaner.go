package amiclean

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"

	"time"
)

const (
	// RFC8601 is the date/time format used by AWS.
	RFC8601 = "2006-01-02T15:04:05.000Z"
)

// AMIClean defines parameters for cleaning up AMIs based on the Branch and
// Expiration Date.
type AMIClean struct {
	Delete         bool
	Branch         string
	Invert         bool
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

// TODO: Need a generic function that will find the value of a given tag key
// for an AMI. This will simplify the code for FindImagesToPurge and also
// make it easier to give the tag output in the PurgeImages function. Making
// it generic will also allow us to change the branch option to a more generic
// tag option.

// FindImagesToPurge looks through the AMIs available and produces a slice of
// ec2.Image objects to put on the chopping block based on the contents of the
// AMIClean struct.
func (a *AMIClean) FindImagesToPurge(output *ec2.DescribeImagesOutput) []*ec2.Image {
	var ImagesToPurge []*ec2.Image
	for _, image := range output.Images {
		imageCreationTime, _ := time.Parse(RFC8601, *image.CreationDate)
		// If the AMI isn't old enough to be purged, we don't care
		// about anything else. Just move on.
		if imageCreationTime.After(a.ExpirationDate) {
			continue
		} else {
			// If invert is set, we're looking for AMIs which are
			// NOT in the branch selected.
			if a.Invert {
				for _, tag := range image.Tags {
					if *tag.Key == "Branch" && *tag.Value != a.Branch {
						// Optimally, this output
						// should all get moved into the
						// PurgeImages call.
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
				// Otherwise, we're looking at all the AMIs with Branch
				// set to whatever we set it to in the command line.
			} else {
				for _, tag := range image.Tags {
					if *tag.Key == "Branch" && *tag.Value == a.Branch {
						// Same note as above.
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
		// This is a circuit breaker because we currently assume all
		// AMIs have EBS volumes. This is the case right now, but it
		// isn't true in a more general case. More functionality would
		// need to be added to handle instance-store backed AMIs.
		if *image.RootDeviceType != "ebs" {
			a.Logger.Info("image root device not EBS; will not purge",
				zap.String("ami-id", *image.ImageId),
			)
		} else {
			// There may be multiple snapshots attached to a single AMI,
			// so we need to build a list and iterate on them.
			var snapshotIds []*string
			for _, blockDevice := range image.BlockDeviceMappings {
				snapshotID := *blockDevice.Ebs.SnapshotId
				snapshotIds = append(snapshotIds, &snapshotID)
			}
			deregisterInput := &ec2.DeregisterImageInput{
				DryRun:  aws.Bool(!a.Delete),
				ImageId: aws.String(*image.ImageId),
			}
			if a.Delete {
				a.Logger.Info("deregistering ami",
					zap.String("ami-id", *image.ImageId),
				)
				_, err := a.EC2Client.DeregisterImage(deregisterInput)
				if err != nil {
					return err
				}
			} else {
				a.Logger.Info("would deregister ami",
					zap.String("ami-id", *image.ImageId),
				)
			}
			for _, snapshot := range snapshotIds {
				deleteInput := &ec2.DeleteSnapshotInput{
					DryRun:     aws.Bool(!a.Delete),
					SnapshotId: aws.String(*snapshot),
				}
				if a.Delete {
					a.Logger.Info("deleting snapshot",
						zap.String("snapshot-id", *deleteInput.SnapshotId),
					)
					_, err := a.EC2Client.DeleteSnapshot(deleteInput)
					if err != nil {
						return err
					}
				} else {
					a.Logger.Info("would delete snapshot",
						zap.String("snapshot-id", *deleteInput.SnapshotId),
					)
				}
			}
		}
	}
	return nil
}
