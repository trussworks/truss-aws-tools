package amiclean

import (
	"testing"
	"time"
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"
)

var newMasterImage = &ec2.Image{
	Description: aws.String("New Master Image"),
	ImageId: aws.String("ami-11111111111111111"),
	CreationDate: aws.String("2019-03-31T21:04:57.000Z"),
	Tags: []*ec2.Tag{
		&ec2.Tag{ Key: aws.String("Branch"), Value: aws.String("master") },
		&ec2.Tag{ Key: aws.String("Name"), Value: aws.String("newMasterImage") },
	},
	BlockDeviceMappings: []*ec2.BlockDeviceMapping{
		&ec2.BlockDeviceMapping{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &ec2.EbsBlockDevice{
				SnapshotId: aws.String("snap-11111111111111111"),
			},
		},
	},
}

var newishDevImage = &ec2.Image{
	Description: aws.String("Newish Dev Image"),
	ImageId: aws.String("ami-22222222222222222"),
	CreationDate: aws.String("2019-03-30T21:04:57.000Z"),
	Tags: []*ec2.Tag{
		&ec2.Tag{ Key: aws.String("Branch"), Value: aws.String("development") },
		&ec2.Tag{ Key: aws.String("Name"), Value: aws.String("newishDevImage") },
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
}

var oldDevImage = &ec2.Image{
	Description: aws.String("Old Dev Image"),
	ImageId: aws.String("ami-33333333333333333"),
	CreationDate: aws.String("2019-03-01T21:04:57.000Z"),
	Tags: []*ec2.Tag{
		&ec2.Tag{ Key: aws.String("Name"), Value: aws.String("oldDevImage") },
		&ec2.Tag{ Key: aws.String("Branch"), Value: aws.String("development") },
	},
	BlockDeviceMappings: []*ec2.BlockDeviceMapping{
		&ec2.BlockDeviceMapping{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &ec2.EbsBlockDevice{
				SnapshotId: aws.String("snap-33333333333333333"),
			},
		},
	},
}

var testImages = []*ec2.Image{ newMasterImage, newishDevImage, oldDevImage }

var now = time.Date(2019, 4, 1, 0, 0, 0, 0, time.UTC)

func TestFindImagesToPurge(t *testing.T) {
	logger, _ := zap.NewProduction()
	tables := []struct {
		imageSet []*ec2.Image
		Branch string
		RetentionDays int
		resultSet []*ec2.Image
	}{
		{ testImages, "master", 1, []*ec2.Image(nil) },
		{ testImages, "development", 30, []*ec2.Image{ oldDevImage } },
		{ testImages, "development", 1, []*ec2.Image{ newishDevImage, oldDevImage } },
		{ testImages, "!master", 1, []*ec2.Image{ newishDevImage, oldDevImage } },
	}

	for _, table := range tables {
		a := AMIClean{
			Branch: table.Branch,
			DryRun: true,
			ExpirationDate: now.AddDate(0, 0, -int(table.RetentionDays)),
			Logger: logger,
			EC2Client: nil,
		}

		output := &ec2.DescribeImagesOutput{ Images: testImages }
		result := a.FindImagesToPurge(output)
		if !reflect.DeepEqual(result, table.resultSet) {
			t.Errorf("ERROR: branch %v, retention %d days failed;\n\texpected: %v\n\tgot: %v",
			table.Branch,
			table.RetentionDays,
			table.resultSet,
			result)
		}
	}
}
