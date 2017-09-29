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
	tagsInput := &ec2.CreateTagsInput{
		Resources: []*string{snapshot.SnapshotId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Test Tag"),
				Value: aws.String("Test Value"),
			},
		},
	}
	_, err = ec2Client.CreateTags(tagsInput)
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
