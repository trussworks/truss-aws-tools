package amiclean

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/trussworks/truss-aws-tools/internal/aws/session"
	"go.uber.org/zap"

	"strings"
	"time"
)

const (
	// RFC8601 is the date/time format used by AWS.
	RFC8601 = "2006-01-02T15:04:05.000Z"
)

// AMIClean defines parameters for cleaning up AMIs based on a tag and
// expiration date.
type AMIClean struct {
	NamePrefix     string
	Delete         bool
	Tag            *ec2.Tag
	Invert         bool
	Unused         bool
	Role           string
	ExpirationDate time.Time
	Logger         *zap.Logger
	EC2Client      *ec2.EC2
	STSClient      *sts.STS
}

// GetImages gets us all the private AMIs on our account so that they can be
// looked through later. We have to do this here because the AWS API does not
// allow you to search for AMIs by creation date or by *not* having a tag set to
// a certain value, which would speed this up considerably.
func (a *AMIClean) GetImages() (*ec2.DescribeImagesOutput, error) {
	var output *ec2.DescribeImagesOutput

	input := &ec2.DescribeImagesInput{
		Owners: []*string{aws.String("self")},
	}

	output, err := a.EC2Client.DescribeImages(input)

	if err != nil {
		return nil, err
	}

	return output, nil
}

// MatchTags lets us see if an arbitrary tag is set to the appropriate value
// within an image.
func matchTags(image *ec2.Image, tag *ec2.Tag) (bool, *ec2.Tag) {
	for _, imageTag := range image.Tags {
		if *tag.Key == "" || *tag.Key == *imageTag.Key {
			if *tag.Value == "" || *tag.Value == *imageTag.Value {
				// If the tag exists, and has the value we're
				// looking for or we don't need match the tag,
				// return true and the image tag.
				return true, imageTag
			}
			// If the tag exists, and doesn't have the
			// value we're looking for, return false and
			// the image tag.
			return false, imageTag

		}
	}

	// If we didn't find the tag key anywhere, return false and a filler
	// value.
	return false, &ec2.Tag{Key: tag.Key, Value: aws.String("not found")}
}

func (a *AMIClean) getImageLaunchPermission(image *ec2.Image) ([]*ec2.LaunchPermission, error) {
	// Create an input into DescribeImageAttributeInput to retrieve image launch permissions.
	describeImageAttributeInput := &ec2.DescribeImageAttributeInput{
		Attribute: aws.String("launchPermission"),
		ImageId:   image.ImageId,
	}
	output, err := a.EC2Client.DescribeImageAttribute(describeImageAttributeInput)
	if err != nil {
		return nil, err
	}
	return output.LaunchPermissions, nil
}

func (a *AMIClean) checkAccountUnused(ec2Client *ec2.EC2, accountID *string, image *ec2.Image) (bool, error) {

	amiFilter := &ec2.Filter{
		Name:   aws.String("image-id"),
		Values: []*string{image.ImageId},
	}

	findInstancesInput := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{amiFilter},
	}

	output, err := ec2Client.DescribeInstances(findInstancesInput)
	if err != nil {
		return false, err
	}

	if output.Reservations != nil {
		a.Logger.Info("found image running instance in aws account",
			zap.String("account-id", *accountID),
			zap.String("ami-id", *image.ImageId),
			zap.String("ami-name", *image.Name),
		)
		return false, nil
	}

	return true, nil
}

// CheckUnused takes an image and then checks to see if it is in use
// on a running ec2 instance. If a cross account sts role has
// been provided, it checks on the image owner and all aws accounts listed in
// the image launch permissions.
// If a cross account sts role hasn't been provided, it checks
// on the image owner account only.
// If the image is in use, it should return false; if it
// is not in use, it should return true.
// TODO: Also check to see if we are using it for any ASG launch
// configurations. This is more difficult because you cannot filter them
// by AMI ID like you can with instances; you have to fetch all of them
// and then parse through them doing the comparison, making it much more
// onerous. :/
func (a *AMIClean) CheckUnused(image *ec2.Image) (bool, error) {

	if a.Role == "" {
		return a.checkAccountUnused(a.EC2Client, image.OwnerId, image)
	}

	imageAccountIDs := []*string{image.OwnerId}

	imageLaunchPermissions, err := a.getImageLaunchPermission(image)
	if err != nil {
		return false, err
	}

	for _, imageLaunchPermission := range imageLaunchPermissions {
		imageAccountIDs = append(imageAccountIDs, imageLaunchPermission.UserId)
	}

	for _, imageAccountID := range imageAccountIDs {
		assumeRoleOutput, err := a.STSClient.AssumeRole(&sts.AssumeRoleInput{
			RoleArn:         aws.String(fmt.Sprintf("arn:aws:iam::%s:role/%s", *imageAccountID, a.Role)),
			RoleSessionName: aws.String(fmt.Sprintf("ami-cleaner-%s", a.NamePrefix)),
		})

		if err != nil {
			return false, err
		}

		stsEC2Client := session.MakeEC2Client(session.MustMakeSessionWithSTSCredentials(assumeRoleOutput.Credentials))

		if result, err := a.checkAccountUnused(stsEC2Client, imageAccountID, image); err != nil || !result {
			return result, err
		}
	}

	return true, nil
}

// CheckImage compares a given image to the purge criteria and returns true
// if the image matches the criteria.
func (a *AMIClean) CheckImage(image *ec2.Image) bool {
	// First look at the name and see if it matches our prefix. If it
	// does not, we can bail out quickly with a false result.
	if !strings.HasPrefix(*image.Name, a.NamePrefix) {
		return false
	}

	// Next, check the image's age and compare it to our expiration date.
	// If it's not old enough, we can again return false.
	imageCreationTime, _ := time.Parse(RFC8601, *image.CreationDate)
	if imageCreationTime.After(a.ExpirationDate) {
		return false
	}

	// If we've gotten this far, we want to see if the "unused" flag was
	// set. If so, we need to see if it's being used.
	if a.Unused {
		unused, err := a.CheckUnused(image)
		if err != nil {
			a.Logger.Error("Could not check for image in use",
				zap.String("ami-id", *image.ImageId),
				zap.String("ami-name", *image.Name),
				zap.Error(err),
			)
			// If errored out, we want to bail out for safety.
			return false
		}
		// If we didn't error out, and the image is being used,
		// we should return false.
		if !unused {
			return false
		}
	}

	// We want to check against the tags we're looking at.
	match, matchedTag := matchTags(image, a.Tag)
	// We can be a little clever here to reduce our code. If a.Invert is
	// not the same as match, then we know either Invert was not set and
	// we do have a match, or Invert was set and we don't have a match;
	// either way, this is an AMI we want to mark for removal.
	if a.Invert != match {
		a.Logger.Debug("ami matched selection criteria",
			zap.String("ami-id", *image.ImageId),
			zap.String("ami-name", *image.Name),
			zap.String("ami-tag-key", *matchedTag.Key),
			zap.String("ami-tag-value", *matchedTag.Value),
			zap.String("ami-creation-date", imageCreationTime.String()),
		)
		return true
	}

	// If we've gotten here, we know the AMI doesn't need to go.
	return false
}

// PurgeImage operates on a single image, registering the image and
// deleting any associated snapshots. We return the ID of the AMI
// we deleted (in case that is interesting) and any errors.
func (a *AMIClean) PurgeImage(image *ec2.Image) (string, error) {
	// This is a circuit breaker because we currently assume all
	// AMIs have EBS volumes. This is the case right now, but it
	// isn't true in a more general case. More functionality would
	// need to be added to handle instance-store backed AMIs.
	if *image.RootDeviceType != "ebs" {
		a.Logger.Info("image root device not EBS; will not purge",
			zap.String("ami-id", *image.ImageId),
			zap.String("ami-name", *image.Name),
		)
	} else {
		// There may be multiple snapshots attached to a single AMI,
		// so we need to build a list and iterate on them.
		var snapshotIds []*string
		for _, blockDevice := range image.BlockDeviceMappings {
			if blockDevice.Ebs != nil {
				snapshotID := *blockDevice.Ebs.SnapshotId
				snapshotIds = append(snapshotIds, &snapshotID)
			}
		}
		deregisterInput := &ec2.DeregisterImageInput{
			DryRun:  aws.Bool(!a.Delete),
			ImageId: aws.String(*image.ImageId),
		}
		if a.Delete {
			a.Logger.Info("deregistering ami",
				zap.String("ami-id", *image.ImageId),
				zap.String("ami-name", *image.Name),
			)
			_, err := a.EC2Client.DeregisterImage(deregisterInput)
			if err != nil {
				return "Failed to deregister image", err
			}
		} else {
			a.Logger.Info("would deregister ami",
				zap.String("ami-id", *image.ImageId),
				zap.String("ami-name", *image.Name),
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
					return "Failed to delete snapshot", err
				}
			} else {
				a.Logger.Info("would delete snapshot",
					zap.String("snapshot-id", *deleteInput.SnapshotId),
				)
			}
		}
	}
	return *image.ImageId, nil
}
