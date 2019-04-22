package amiclean

import (
	"reflect"
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
		&ec2.Tag{Key: aws.String("Branch"), Value: aws.String("master")},
		&ec2.Tag{Key: aws.String("Name"), Value: aws.String("newMasterImage")},
	},
	BlockDeviceMappings: []*ec2.BlockDeviceMapping{
		&ec2.BlockDeviceMapping{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &ec2.EbsBlockDevice{
				SnapshotId: aws.String("snap-11111111111111111"),
			},
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
		&ec2.Tag{Key: aws.String("Branch"), Value: aws.String("development")},
		&ec2.Tag{Key: aws.String("Name"), Value: aws.String("newishDevImage")},
	},
	BlockDeviceMappings: []*ec2.BlockDeviceMapping{
		&ec2.BlockDeviceMapping{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &ec2.EbsBlockDevice{
				SnapshotId: aws.String("snap-22222222222222222"),
			},
		},
		&ec2.BlockDeviceMapping{
			DeviceName: aws.String("/dev/xvdb"),
			Ebs: &ec2.EbsBlockDevice{
				SnapshotId: aws.String("snap-22222222222222223"),
			},
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
		&ec2.Tag{Key: aws.String("Name"), Value: aws.String("oldDevImage")},
		&ec2.Tag{Key: aws.String("Branch"), Value: aws.String("development")},
		&ec2.Tag{Key: aws.String("Foozle"), Value: aws.String("Fizzbin")},
	},
	BlockDeviceMappings: []*ec2.BlockDeviceMapping{
		&ec2.BlockDeviceMapping{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &ec2.EbsBlockDevice{
				SnapshotId: aws.String("snap-33333333333333333"),
			},
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
		&ec2.Tag{Key: aws.String("Name"), Value: aws.String("noEbsImage")},
		&ec2.Tag{Key: aws.String("Branch"), Value: aws.String("experimental")},
		&ec2.Tag{Key: aws.String("Foozle"), Value: aws.String("Whatsit")},
	},
	RootDeviceType: aws.String("instance-store"),
}

var testImages = []*ec2.Image{newMasterImage, newishDevImage, oldDevImage, noEbsImage}

var now = time.Date(2019, 4, 1, 0, 0, 0, 0, time.UTC)

var logger, _ = zap.NewProduction()

func TestFindImagesToPurge(t *testing.T) {
	tables := []struct {
		imageSet      []*ec2.Image
		NamePrefix    string
		Tag           *ec2.Tag
		Invert        bool
		RetentionDays int
		resultSet     []*ec2.Image
	}{
		{testImages, "", &ec2.Tag{ Key: aws.String("Branch"), Value: aws.String("master")}, false, 1, []*ec2.Image(nil)},
		{testImages, "", &ec2.Tag{ Key: aws.String("Branch"), Value: aws.String("development")}, false, 30, []*ec2.Image{oldDevImage}},
		{testImages, "", &ec2.Tag{ Key: aws.String("Branch"), Value: aws.String("development")}, false, 1, []*ec2.Image{newishDevImage, oldDevImage}},
		{testImages, "", &ec2.Tag{ Key: aws.String("Branch"), Value: aws.String("master")}, true, 1, []*ec2.Image{newishDevImage, oldDevImage, noEbsImage}},
		{testImages, "devimage", &ec2.Tag{ Key: aws.String("Branch"), Value: aws.String("master")}, true, 1, []*ec2.Image{newishDevImage, oldDevImage}},
		{testImages, "", &ec2.Tag{ Key: aws.String("Foozle"), Value: aws.String("Whatsit")}, false, 1, []*ec2.Image{noEbsImage}},
		{testImages, "", &ec2.Tag{ Key: aws.String("Foozle"), Value: aws.String("Whatsit")}, true, 0, []*ec2.Image{newMasterImage, newishDevImage, oldDevImage}},
	}

	for _, table := range tables {
		a := AMIClean{
			NamePrefix:	table.NamePrefix,
			Tag:            table.Tag,
			Invert:         table.Invert,
			Delete:         false,
			ExpirationDate: now.AddDate(0, 0, -int(table.RetentionDays)),
			Logger:         logger,
			EC2Client:      nil,
		}

		output := &ec2.DescribeImagesOutput{Images: testImages}
		result := a.FindImagesToPurge(output)
		var result_names []string
		for _, result_item := range result {
			result_names = append(result_names, *result_item.Name)
		}
		if !reflect.DeepEqual(result, table.resultSet) {
			t.Errorf("ERROR: prefix: %v, branch %v, invert %v, retention %v;\n\texpected: %v\n\tgot: %v",
				table.NamePrefix,
				table.Tag,
				table.Invert,
				table.RetentionDays,
				table.resultSet,
				result_names)
		}
	}
}

// Actually purging images is a little difficult to test; the function always
// returns nil. We might want to change that so it can be tested. I am adding
// this test to at least see the messages and know that all the logic is
// working right.
func TestPurgeImages(t *testing.T) {
	a := AMIClean{
		Tag:            &ec2.Tag{ Key: aws.String("Branch"), Value: aws.String("master")},
		Invert:         true,
		Delete:         false,
		ExpirationDate: now.AddDate(0, 0, -1),
		Logger:         logger,
		EC2Client:      nil,
	}

	err := a.PurgeImages(testImages)
	if err != nil {
		t.Errorf("ERROR: PurgeImages test failed")
	}
}
