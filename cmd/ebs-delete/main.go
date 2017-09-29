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
	fmt.Println(res.Volumes[0])
	// Generate name of snapshot.
	// Get tags to copy over.
	// Make snapshot.
	// Apply tags.
}
