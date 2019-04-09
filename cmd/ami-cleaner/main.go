package main

import (
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/ec2"

    "fmt"
    "time"
    //"errors"
)

const (
	// RFC8601 is the date/time format used by AWS.
	RFC8601 = "2006-01-02T15:04:05-07:00"
)

// This is the day beyond which we don't need to keep things (1 day ago)
var ExpirationDate time.Time = time.Now().AddDate(0, 0, -1)

// So, we need a function that can take a look at the images in some output
// from DescribeImages and return us the images that match our conditions.
func FindImagesToPurge(output *ec2.DescribeImagesOutput) []*ec2.Image {
    var ImagesToPurge []*ec2.Image
    for _, image := range output.Images {
	    v := *image.CreationDate
	    imageCreationTime, _ := time.Parse(RFC8601, v)
	    for _, tag := range image.Tags {
		    if *tag.Key == "Branch" && *tag.Value != "master" &&
		    imageCreationTime.Before(ExpirationDate) {
			    ImagesToPurge = append(ImagesToPurge, image)
		    }
	    }
    }
    return ImagesToPurge
}

// This function takes a slice of Images, and then finds the AMI IDs and
// snapshot IDs associated with them.
func GetIdsForProcessing(images []*ec2.Image) ([]string, []string) {
	var amiIds, snapshotIds []string
	for _, image := range images {
		amiId := *image.ImageId
		amiIds = append(amiIds, amiId)
		for _, blockDevice := range image.BlockDeviceMappings {
			snapshotId := *blockDevice.Ebs.SnapshotId
			snapshotIds = append(snapshotIds, snapshotId)
		}
	}
	return amiIds, snapshotIds
}

// This function takes a list of AMI IDs (as strings) and runs the deregister
// operation on each one.
func deregisterImageList(ec2conn *ec2.EC2, imageList []string) error {
	for _, amiId := range imageList {
		deregisterInput := &ec2.DeregisterImageInput{
			DryRun: aws.Bool(true),
			ImageId: aws.String(amiId),
		}
		fmt.Println("Attempting to deregister", amiId)
		output, err := ec2conn.DeregisterImage(deregisterInput)
		fmt.Println(output)
		if err != nil {
			fmt.Println(err)
			// return err
		}
	}
	return nil
}

// This function takes a list of snapshot IDs (as strings) and runs the
// delete operation on each one.
func deleteSnapshotList(ec2conn *ec2.EC2, snapshotList []string) error {
	for _, snapshotId := range snapshotList {
		deleteInput := &ec2.DeleteSnapshotInput{
			DryRun: aws.Bool(true),
			SnapshotId: aws.String(snapshotId),
		}
		fmt.Println("Attempting to delete", snapshotId)
		output, err := ec2conn.DeleteSnapshot(deleteInput)
		fmt.Println(output)
		if err != nil {
			fmt.Println(err)
			// return err
		}
	}
	return nil
}


func main() {
    // This creates "sess", which is essentially a collection of credentials,
    // a region, etc needed for establishing a connection to an AWS endpoint.
    sess := session.Must(session.NewSessionWithOptions(session.Options{
	SharedConfigState: session.SharedConfigEnable,
    }))

    // Now we're going to create a new connection to EC2 named "ec2Svc" that
    // uses the credentials and info we passed from sess above.
    ec2Svc := ec2.New(sess)

    // So now we reach the real conundrum. DescribeImages does not let us
    // filter out images that *don't* match a certain tag (everything not
    // off the master branch, for instance) and it doesn't let us get them
    // by age either.

    // So what can we do to reduce the number of things we have to look at?
    // Well, we can toss out anything public at least, so that should get us
    // only the AMIs we're generating.
    input := &ec2.DescribeImagesInput{
        Filters: []*ec2.Filter{
            {
                Name: aws.String("is-public"),
		Values : []*string{aws.String("false")},
	    },
	},
    }

    // Now we grab everything the matches the conditions above. This is an
    // EC2DescribeImagesOutput type, which consists of a slice of Image
    // types.
    result, err := ec2Svc.DescribeImages(input)

    if err != nil {
        fmt.Println(err.Error())
	return
    }

    // Now we have the list of images; let's put them through the filter
    // and get the ones that meet our conditions.
    purgelist := FindImagesToPurge(result)

    // Now we have a list of images to purge; we need two things from each
    // image; the ImageId and any BlockDeviceMappings.Ebs.SnapshotId elements
    // (they need to be deleted after we deregister the AMI).
    amiIds, snapshotIds := GetIdsForProcessing(purgelist)

    // Now we have a list of AMIs and snapshots to get deleted; we need to
    // deregister AMIs first, then delete the snapshots.
    deregisterImageList(ec2Svc, amiIds)

    // And now that the AMIs have been registered, we can delete the snapshots.
    deleteSnapshotList(ec2Svc, snapshotIds)

}
