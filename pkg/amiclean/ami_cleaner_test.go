package amiclean

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"
)

var newMasterImage = &ec2.Image{
	Name:         aws.String("masterimage-alpha"),
	Description:  aws.String("New Master Image"),
	ImageId:      aws.String("ami-11111111111111111"),
	CreationDate: aws.String("2019-03-31T21:04:57.000Z"),
	Tags: []*ec2.Tag{
		{Key: aws.String("Branch"), Value: aws.String("master")},
		{Key: aws.String("Name"), Value: aws.String("newMasterImage")},
	},
	BlockDeviceMappings: []*ec2.BlockDeviceMapping{
		{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &ec2.EbsBlockDevice{
				SnapshotId: aws.String("snap-11111111111111111"),
			},
		},
		{
			DeviceName:  aws.String("/dev/sdb"),
			VirtualName: aws.String("ephemeral0"),
		},
	},
	RootDeviceType: aws.String("ebs"),
}

var newishDevImage = &ec2.Image{
	Name:         aws.String("devimage-alpha"),
	Description:  aws.String("Newish Dev Image"),
	ImageId:      aws.String("ami-22222222222222222"),
	CreationDate: aws.String("2019-03-30T21:04:57.000Z"),
	Tags: []*ec2.Tag{
		{Key: aws.String("Branch"), Value: aws.String("development")},
		{Key: aws.String("Name"), Value: aws.String("newishDevImage")},
	},
	BlockDeviceMappings: []*ec2.BlockDeviceMapping{
		{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &ec2.EbsBlockDevice{
				SnapshotId: aws.String("snap-22222222222222222"),
			},
		},
		{
			DeviceName: aws.String("/dev/xvdb"),
			Ebs: &ec2.EbsBlockDevice{
				SnapshotId: aws.String("snap-22222222222222223"),
			},
		},
		{
			DeviceName:  aws.String("/dev/sdb"),
			VirtualName: aws.String("ephemeral0"),
		},
		{
			DeviceName:  aws.String("/dev/sdc"),
			VirtualName: aws.String("ephemeral1"),
		},
	},
	RootDeviceType: aws.String("ebs"),
}

var oldDevImage = &ec2.Image{
	Name:         aws.String("devimage-bravo"),
	Description:  aws.String("Old Dev Image"),
	ImageId:      aws.String("ami-33333333333333333"),
	CreationDate: aws.String("2019-03-01T21:04:57.000Z"),
	Tags: []*ec2.Tag{
		{Key: aws.String("Name"), Value: aws.String("oldDevImage")},
		{Key: aws.String("Branch"), Value: aws.String("development")},
		{Key: aws.String("Foozle"), Value: aws.String("Fizzbin")},
	},
	BlockDeviceMappings: []*ec2.BlockDeviceMapping{
		{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &ec2.EbsBlockDevice{
				SnapshotId: aws.String("snap-33333333333333333"),
			},
		},
		{
			DeviceName:  aws.String("/dev/sdb"),
			VirtualName: aws.String("ephemeral0"),
		},
	},
	RootDeviceType: aws.String("ebs"),
}

var noEbsImage = &ec2.Image{
	Name:         aws.String("experiment-alpha"),
	Description:  aws.String("No EBS Image"),
	ImageId:      aws.String("ami-44444444444444444"),
	CreationDate: aws.String("2019-03-01T21:04:57.000Z"),
	Tags: []*ec2.Tag{
		{Key: aws.String("Name"), Value: aws.String("noEbsImage")},
		{Key: aws.String("Branch"), Value: aws.String("experimental")},
		{Key: aws.String("Foozle"), Value: aws.String("Whatsit")},
	},
	RootDeviceType: aws.String("instance-store"),
}

var noTagImage = &ec2.Image{
	Name:         aws.String("notagimage-beta"),
	Description:  aws.String("No Tag Image"),
	ImageId:      aws.String("ami-555555555555555555"),
	CreationDate: aws.String("2019-03-01T21:04:57.000Z"),
	Tags:         nil,
	BlockDeviceMappings: []*ec2.BlockDeviceMapping{
		{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &ec2.EbsBlockDevice{
				SnapshotId: aws.String("snap-55555555555555555"),
			},
		},
	},
	RootDeviceType: aws.String("ebs"),
}

var testImages = []*ec2.Image{newMasterImage, newishDevImage, oldDevImage, noEbsImage, noTagImage}

var now = time.Date(2019, 4, 1, 0, 0, 0, 0, time.UTC)

var logger, _ = zap.NewProduction()

func TestCheckImage(t *testing.T) {
	tables := []struct {
		imageSet      []*ec2.Image
		NamePrefix    string
		Tag           *ec2.Tag
		Invert        bool
		RetentionDays int
		resultSet     []bool
	}{
		{testImages, "", &ec2.Tag{Key: aws.String("Branch"), Value: aws.String("master")}, false, 1, []bool{false, false, false, false, false}},
		{testImages, "", &ec2.Tag{Key: aws.String("Branch"), Value: aws.String("development")}, false, 30, []bool{false, false, true, false, false}},
		{testImages, "", &ec2.Tag{Key: aws.String("Branch"), Value: aws.String("development")}, false, 1, []bool{false, true, true, false, false}},
		{testImages, "", &ec2.Tag{Key: aws.String("Branch"), Value: aws.String("master")}, true, 1, []bool{false, true, true, true, true}},
		{testImages, "devimage", &ec2.Tag{Key: aws.String("Branch"), Value: aws.String("master")}, true, 1, []bool{false, true, true, false, false}},
		{testImages, "", &ec2.Tag{Key: aws.String("Foozle"), Value: aws.String("Whatsit")}, false, 1, []bool{false, false, false, true, false}},
		{testImages, "", &ec2.Tag{Key: aws.String("Foozle"), Value: aws.String("Whatsit")}, true, 0, []bool{true, true, true, false, true}},
		{testImages, "notagimage", &ec2.Tag{Key: aws.String(""), Value: aws.String("")}, true, 0, []bool{false, false, false, false, true}},
		{testImages, "", &ec2.Tag{Key: aws.String(""), Value: aws.String("")}, false, 1, []bool{false, true, true, true, false}},
		{testImages, "testimage", &ec2.Tag{Key: aws.String(""), Value: aws.String("")}, false, 10, []bool{false, false, false, false, false}},
	}

	for _, table := range tables {
		a := AMIClean{
			NamePrefix:     table.NamePrefix,
			Tag:            table.Tag,
			Invert:         table.Invert,
			Delete:         false,
			ExpirationDate: now.AddDate(0, 0, -int(table.RetentionDays)),
			Logger:         logger,
			EC2Client:      nil,
		}

		for index, image := range testImages {
			if a.CheckImage(image) != table.resultSet[index] {
				t.Errorf("ERROR: prefix: %v, tag: %v, invert %v, retention %v, image %v;\n\texpected: %v\n\tgot: %v",
					table.NamePrefix,
					table.Tag,
					table.Invert,
					table.RetentionDays,
					*image.Name,
					table.resultSet,
					a.CheckImage(image),
				)
			}
		}
	}
}

// Testing the image purging is a little difficult; since we're not acting
// on the actual AWS API, it's probably not going to error out. But this
// does at least ensure that we're acting on the right types and parsing
// things correctly, and we can see the log messages from the tests.
func TestPurgeImage(t *testing.T) {
	a := AMIClean{
		Tag:            &ec2.Tag{Key: aws.String("Branch"), Value: aws.String("master")},
		Invert:         true,
		Delete:         false,
		ExpirationDate: now.AddDate(0, 0, -1),
		Logger:         logger,
		EC2Client:      nil,
		STSClient:      nil,
	}

	for _, image := range testImages {
		deletedImage, err := a.PurgeImage(image)
		if !(deletedImage == *image.ImageId && err == nil) {
			t.Errorf("ERROR: PurgeImage test failed for %v", *image.ImageId)
		}
	}
}
