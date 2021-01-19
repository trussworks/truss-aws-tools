package ebsclean

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"
)

const (
	// RFC8601 is the date/time format used by AWS.
	RFC8601 = "2006-01-02T15:04:05-07:00"
)

// EBSSnapshotClean defines parameters for cleaning EBS snapshots
// based on ExpirationDate
type EBSSnapshotClean struct {
	DryRun         bool
	ExpirationDate time.Time
	Logger         *zap.Logger
	EC2Client      *ec2.EC2
}

// GetEBSSnapshots will return a slice of DB snapshots to delete
func (e *EBSSnapshotClean) GetEBSSnapshots() ([]*ec2.Snapshot, error) {

	input := &ec2.DescribeSnapshotsInput{
		OwnerIds: []*string{aws.String("self")},
	}

	output, err := e.EC2Client.DescribeSnapshots(input)

	if err != nil {
		return nil, err
	}

	return output.Snapshots, nil
}

// CheckEBSSnapshot checks if an ebs snapshot is a candidate for deletion
func (e *EBSSnapshotClean) CheckEBSSnapshot(snapshot *ec2.Snapshot) bool {
	// Next, check the snapshot's age and compare it to our expiration date.
	// If it's not old enough, we can return false.
	snapshotCreationTime := *snapshot.StartTime
	return !snapshotCreationTime.After(e.ExpirationDate)
}

// DeleteEBSSnapshot deletes ebs snapshot and waits for it to complete
func (e *EBSSnapshotClean) DeleteEBSSnapshot(snapshotID *string) error {
	input := &ec2.DeleteSnapshotInput{
		DryRun:     aws.Bool(e.DryRun),
		SnapshotId: aws.String(*snapshotID),
	}

	if e.DryRun {
		return nil
	}

	_, err := e.EC2Client.DeleteSnapshot(input)

	if err != nil {
		return err
	}

	return nil
}
