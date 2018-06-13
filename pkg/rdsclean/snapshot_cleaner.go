package rdsclean

import (
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"go.uber.org/zap"
)

const (
	// RFC8601 is the date/time format used by AWS.
	RFC8601 = "2006-01-02T15:04:05-07:00"
)

// RDSManualSnapshotClean defines parameters for cleaning manual RDS snapshots
// based on ExpirationDate and MaxDBSnapshotCount
type RDSManualSnapshotClean struct {
	DBInstanceIdentifier string
	DryRun               bool
	ExpirationDate       time.Time
	Logger               *zap.Logger
	MaxDBSnapshotCount   uint
	RDSClient            *rds.RDS
}

// FindDBSnapshotsToDelete will return a slice of DB snapshots to delete
func (r *RDSManualSnapshotClean) FindDBSnapshotsToDelete(dbSnapshots []*rds.DBSnapshot) ([]*rds.DBSnapshot, error) {
	var dbSnapshotsToDelete []*rds.DBSnapshot

	sortDBSnapshots(dbSnapshots)
	for i, s := range dbSnapshots {
		// add snapshot to delete slice if past expiration
		if s.SnapshotCreateTime.Before(r.ExpirationDate) {
			dbSnapshotsToDelete = append(dbSnapshotsToDelete, s)
			continue
		}
		// if we are still over maxDBSnapshots add to the delete slice
		// skip if maxDBSnapshotsCount is 0
		if i+1 > int(r.MaxDBSnapshotCount) && r.MaxDBSnapshotCount != 0 {
			dbSnapshotsToDelete = append(dbSnapshotsToDelete, s)
		}

	}

	return dbSnapshotsToDelete, nil
}

// FindManualDBSnapshots returns a slice of available manual snapshots
func (r *RDSManualSnapshotClean) FindManualDBSnapshots() ([]*rds.DBSnapshot, error) {
	var manualDBSnapshots []*rds.DBSnapshot

	input := &rds.DescribeDBSnapshotsInput{
		DBInstanceIdentifier: aws.String(r.DBInstanceIdentifier),
		IncludePublic:        aws.Bool(false),
		IncludeShared:        aws.Bool(false),
		SnapshotType:         aws.String("manual"),
	}

	res, err := r.RDSClient.DescribeDBSnapshots(input)
	if err != nil {
		return nil, err
	}

	for _, s := range res.DBSnapshots {
		if s.Status == aws.String("available") || s.SnapshotCreateTime != nil {
			manualDBSnapshots = append(manualDBSnapshots, s)
		}
	}

	return manualDBSnapshots, err
}

// sortDBSnapshots sorts a slice of DB snapshots in chronological order(newest first) using SnapshotCreateTime
func sortDBSnapshots(dbSnapshots []*rds.DBSnapshot) {
	// sort by snapshot creation time
	sort.Slice(dbSnapshots, func(i, j int) bool {
		return dbSnapshots[i].SnapshotCreateTime.After(*dbSnapshots[j].SnapshotCreateTime)
	})
}

// DeleteDBSnapshots iterates through a list of snapshots and calls deleteDBSnapshot
func (r *RDSManualSnapshotClean) DeleteDBSnapshots(dbSnapshotsToDelete []*rds.DBSnapshot) error {
	r.Logger.Info("db snapshots to delete", zap.Int("snapshots", len(dbSnapshotsToDelete)))
	for _, e := range dbSnapshotsToDelete {
		if r.DryRun {
			r.Logger.Info("would delete db snapshot",
				zap.String("db-snapshot-identifier", *e.DBSnapshotIdentifier),
				zap.String("db-snapshot-create-time", e.SnapshotCreateTime.Format(RFC8601)),
			)

		} else {
			r.Logger.Info("deleting snapshot",
				zap.String("db-snapshot-identifier", *e.DBSnapshotIdentifier),
				zap.String("db-snapshot-create-time", e.SnapshotCreateTime.Format(RFC8601)),
			)
			err := r.DeleteDBSnapshot(*e.DBSnapshotIdentifier)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// DeleteDBSnapshot deletes DB snapshot and waits for it to complete
func (r *RDSManualSnapshotClean) DeleteDBSnapshot(DBSnapshotIdentifier string) error {
	deleteDBSnapshotInput := &rds.DeleteDBSnapshotInput{
		DBSnapshotIdentifier: aws.String(DBSnapshotIdentifier),
	}
	_, err := r.RDSClient.DeleteDBSnapshot(deleteDBSnapshotInput)
	if err != nil {
		return err
	}

	WaitUntilDBSnapshotDeletedInput := &rds.DescribeDBSnapshotsInput{
		DBSnapshotIdentifier: aws.String(DBSnapshotIdentifier),
	}
	err = r.RDSClient.WaitUntilDBSnapshotDeleted(WaitUntilDBSnapshotDeletedInput)
	return err
}
