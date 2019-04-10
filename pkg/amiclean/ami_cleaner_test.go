package amiclean

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"
)

// Dummy data:
//   - new AMI in master branch
//   - new-ish AMI in development branch
//   - old AMI in development branch
//   - one must have multiple tags
//   - one must have multiple EBS volumes

var newMasterImage = &ec2.Image{
	Description:
	ImageId: "ami-11111111111111111"
	CreationDate:
	Tags:
}

var newishDevImage = &ec2.Image{
	ImageId: "ami-22222222222222222"
}

var oldDevImage = &ec2.Image{
	ImageId: "ami-33333333333333333"
}

// Need to test FindImagesToPurge with:
// Branch: master, retention 30 days
// Branch: development, retention 30 days
// Branch: development, retention 1 day
// Branch: !master, retention 1 day

// Need to test GetIdsToProcess against dummy data and generate proper list of
// AMI IDs and Snapshot IDs
