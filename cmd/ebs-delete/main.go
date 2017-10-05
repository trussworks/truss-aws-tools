package main

import (
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"log"
	"strings"
)

func main() {
	var volumeID string
	dryRun := false
	flag.StringVar(&volumeID, "volume-id",
		"",
		"The EBS volumeId to delete")
	flag.BoolVar(&dryRun, "dry-run", false,
		"Don't make any changes and log what would have happened.")
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
	if len(res.Volumes) != 1 {
		log.Fatal("No volumes found with volumeId:", volumeID)
	}
	volume := res.Volumes[0]
	cloudformed, stackName := isCloudFormed(volume)
	if cloudformed {
		log.Println("Volume is cloudformed. Delete stack:", stackName)
		return
	}
	snapshotInput := &ec2.CreateSnapshotInput{
		Description: volume.VolumeId,
		VolumeId:    volume.VolumeId,
	}
	var snapshot *ec2.Snapshot
	if dryRun {
		log.Println("Creating snapshot for volumeId:",
			*volume.VolumeId)
	} else {
		snapshot, err = ec2Client.CreateSnapshot(snapshotInput)
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
	}
	deleteVolumeInput := &ec2.DeleteVolumeInput{
		VolumeId: volume.VolumeId,
	}

	if dryRun {
		log.Println("Deleting volume:", *volume.VolumeId)
	} else {
		_, err = ec2Client.DeleteVolume(deleteVolumeInput)
		if err != nil {
			log.Fatal(err)
		}
	}
	if !dryRun {
		fmt.Println(volume)
		fmt.Println(snapshot)
	}
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
