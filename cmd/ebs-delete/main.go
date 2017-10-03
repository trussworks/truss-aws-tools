package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"log"
)

func main() {
	var volumeID string
	flag.StringVar(&volumeID, "volume-id",
		"",
		"The EBS volumeId to delete")
	flag.Parse()
	if volumeID == "" {
		flag.PrintDefaults()
		return
	}
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-west-2"),
	})
	if err != nil {
		log.Fatal(err)
	}
	ec2Client := ec2.New(sess)
	// volume-id - The volume ID.
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
	res, err := ec2Client.DescribeVolumes(input)
	if err != nil {
		log.Fatal(err)
	}
	volume := res.Volumes[0]
	snapshotInput := &ec2.CreateSnapshotInput{
		Description: volume.VolumeId,
		VolumeId:    volume.VolumeId,
	}
	snapshot, err := ec2Client.CreateSnapshot(snapshotInput)
	if err != nil {
		log.Fatal(err)
	}
	tags := copyTags(volume.Tags)
	tags = append(tags, &ec2.Tag{
		Key:   aws.String("Name"),
		Value: volume.VolumeId,
	})
	tagsInput := &ec2.CreateTagsInput{
		Resources: []*string{snapshot.SnapshotId},
		Tags:      tags,
	}
	_, err = ec2Client.CreateTags(tagsInput)
	if err != nil {
		log.Fatal(err)
	}
	describeSnapshotsInput := &ec2.DescribeSnapshotsInput{
		SnapshotIds: []*string{snapshot.SnapshotId},
	}
	err = ec2Client.WaitUntilSnapshotCompleted(describeSnapshotsInput)
	if err != nil {
		log.Fatal(err)
	}

	deleteVolumeInput := &ec2.DeleteVolumeInput{
		VolumeId: volume.VolumeId,
	}
	_, err = ec2Client.DeleteVolume(deleteVolumeInput)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(volume)
	fmt.Println(snapshot)
	// Generate name of snapshot.
	// Get tags to copy over.
	// Make snapshot.
	// Apply tags.
}

// copyTags takes a slice of Tags, and copys then into a new slice,
// pre-pending the Keys with X-.
func copyTags(tags []*ec2.Tag) []*ec2.Tag {
	retval := make([]*ec2.Tag, len(tags))
	for i, tag := range tags {
		newTag := ec2.Tag{
			Key:   aws.String("X-" + *tag.Key),
			Value: tag.Value,
		}
		retval[i] = &newTag
	}
	return retval
}
