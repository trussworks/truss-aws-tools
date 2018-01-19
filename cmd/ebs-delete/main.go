package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/trussworks/truss-aws-tools/internal/aws/session"
)

func main() {
	var volumeID, profile, region string
	dryRun := false
	force := false
	flag.StringVar(&volumeID, "volume-id",
		"",
		"The EBS volumeId to delete")
	flag.StringVar(&region, "region", "", "The AWS region to use.")
	flag.StringVar(&profile, "profile", "", "The AWS profile to use.")
	flag.BoolVar(&dryRun, "dry-run", false,
		"Don't make any changes and log what would have happened.")
	flag.BoolVar(&force, "force", false,
		"Delete the volume even if it's part of a CloudFormation stack.")
	flag.Parse()
	if volumeID == "" {
		flag.PrintDefaults()
		return
	}
	ec2Client := makeEC2Client(region, profile)
	volume, err := findVolume(ec2Client, volumeID)
	if err != nil {
		log.Fatal(err)
	}
	cloudformed, stackName := isCloudFormed(volume)
	if cloudformed && !force {
		log.Println("Volume is cloudformed. Delete stack:", stackName)
		return
	}
	var snapshot *ec2.Snapshot
	if dryRun {
		log.Println("Creating snapshot for volumeId:",
			*volume.VolumeId)
	} else {
		snapshot, err = createSnapshotAndWaitUntilCompleted(ec2Client,
			volume)
		if err != nil {
			log.Fatal(err)
		}
	}
	if dryRun {
		log.Println("Deleting volume:", *volume.VolumeId)
	} else {
		err = deleteVolume(ec2Client, volume)
		if err != nil {
			log.Fatal(err)
		}
	}
	if !dryRun {
		fmt.Println(volume)
		fmt.Println(snapshot)
	}
}

// makeEC2Client makes an EC2 client
func makeEC2Client(region, profile string) *ec2.EC2 {
	sess := session.MustMakeSession(region, profile)
	ec2Client := ec2.New(sess)
	return ec2Client
}

// createSnapshotAndWaitUntilCompleted takes a snapshot of an EC2 volume,
// and returns when the snapshot has completed.
func createSnapshotAndWaitUntilCompleted(client *ec2.EC2,
	volume *ec2.Volume) (snapshot *ec2.Snapshot, err error) {
	snapshotInput := &ec2.CreateSnapshotInput{
		Description: volume.VolumeId,
		VolumeId:    volume.VolumeId,
	}
	snapshot, err = client.CreateSnapshot(snapshotInput)
	if err != nil {
		return snapshot, err
	}
	err = copyVolumeTagsToSnapshot(client, volume, snapshot)
	if err != nil {
		return snapshot, err
	}
	describeSnapshotInput := &ec2.DescribeSnapshotsInput{
		SnapshotIds: []*string{snapshot.SnapshotId},
	}
	err = client.WaitUntilSnapshotCompleted(describeSnapshotInput)
	return snapshot, err
}

// copyVolumeTagsToSnapshot copies all tags from the volume to the snapshot
func copyVolumeTagsToSnapshot(client *ec2.EC2,
	volume *ec2.Volume,
	snapshot *ec2.Snapshot) (err error) {
	if volume.Tags != nil {
		tags := copyTags(volume.Tags)
		tagsInput := &ec2.CreateTagsInput{
			Resources: []*string{snapshot.SnapshotId},
			Tags:      tags,
		}
		_, err := client.CreateTags(tagsInput)
		return err
	}
	return nil
}

// findVolume gets a Volume from a volumeid
func findVolume(client *ec2.EC2, volumeID string) (*ec2.Volume, error) {
	input := &ec2.DescribeVolumesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("volume-id"),
				Values: []*string{
					&volumeID,
				},
			},
		},
	}
	res, err := client.DescribeVolumes(input)
	if err != nil {
		return nil, err
	}
	if len(res.Volumes) != 1 {
		return nil, errors.New("No volumes found with volumeId: " + volumeID)
	}
	return res.Volumes[0], nil
}

// deleteVolume deletes the volume
func deleteVolume(client *ec2.EC2, volume *ec2.Volume) (err error) {
	deleteVolumeInput := &ec2.DeleteVolumeInput{
		VolumeId: volume.VolumeId,
	}
	_, err = client.DeleteVolume(deleteVolumeInput)
	return err
}

// copyTags takes a slice of Tags, and copys then into a new slice,
// pre-pending the AWS owned Keys with X-.
func copyTags(tags []*ec2.Tag) []*ec2.Tag {
	retval := make([]*ec2.Tag, len(tags))
	for i, tag := range tags {
		// AWS claims ownership of any tag that starts with aws:
		// so you can't just copy them across. Prefix with X-
		// so we can still find them.
		if strings.HasPrefix(*tag.Key, "aws:") {
			newTag := ec2.Tag{
				Key:   aws.String("X-" + *tag.Key),
				Value: tag.Value,
			}
			retval[i] = &newTag
		} else {
			retval[i] = tag
		}
	}
	return retval
}

func isCloudFormed(volume *ec2.Volume) (ok bool, stackName string) {
	for _, tag := range volume.Tags {
		if *tag.Key == "aws:cloudformation:stack-name" {
			return true, *tag.Value
		}
	}
	return false, ""
}
