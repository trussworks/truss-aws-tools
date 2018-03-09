package main

import (
	"flag"
	"log"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/trussworks/truss-aws-tools/internal/aws/session"
)

const (
	// RFC8601 is the date/time format used by AWS.
	RFC8601 = "2006-01-02T15:04:05-07:00"
)

func main() {
	var dbInstanceIdentifier, profile, region string
	var retentionDays, maxDBSnapshotCount int
	dryRun := false

	flag.StringVar(&dbInstanceIdentifier, "db-instance-identifier",
		"",
		"The RDS database instance identifier.")
	flag.IntVar(&retentionDays, "retention-days",
		30,
		"The maximum retention age in days.")
	flag.IntVar(&maxDBSnapshotCount, "max-snapshots",
		0,
		"The maximum number of manual snapshots allowed. This takes precedence over -retention-days.")
	flag.StringVar(&region, "region", "", "The AWS region to use.")
	flag.StringVar(&profile, "profile", "", "The AWS profile to use.")
	flag.BoolVar(&dryRun, "dry-run", false,
		"Don't make any changes and log what would have happened.")
	flag.Parse()

	if dbInstanceIdentifier == "" {
		log.Fatal("DB instance identifier is required")
	}

	if maxDBSnapshotCount < 0 {
		log.Fatal("max-snapshots must be greater than 0")
	}
	rdsClient := makeRDSClient(region, profile)
	// Snapshots creation time is UTC
	// https://docs.aws.amazon.com/sdk-for-go/api/service/rds/#DBSnapshot
	now := time.Now().UTC()
	expirationDate := now.AddDate(0, 0, -retentionDays)

	manualDBSnapshots, err := findManualDBSnapshots(rdsClient, dbInstanceIdentifier)
	if err != nil {
		log.Fatal(err)
	}

	dbSnapshotsToDelete, err := findDBSnapshotsToDelete(manualDBSnapshots, expirationDate, maxDBSnapshotCount)
	if err != nil {
		log.Fatal(err)
	}

	err = deleteDBSnapshots(rdsClient, dbSnapshotsToDelete, dryRun)
	if err != nil {
		log.Fatal(err)
	}

}

// makeRDSClient makes an RDS client
func makeRDSClient(region, profile string) *rds.RDS {
	sess := session.MustMakeSession(region, profile)
	rdsClient := rds.New(sess)
	return rdsClient
}

// findDBSnapshotsToDelete will return a slice of DB snapshots to delete
func findDBSnapshotsToDelete(dbSnapshots []*rds.DBSnapshot, expirationDate time.Time, maxDBSnapshotCount int) ([]*rds.DBSnapshot, error) {
	var dbSnapshotsToDelete []*rds.DBSnapshot

	sortDBSnapshots(dbSnapshots)
	for i, s := range dbSnapshots {
		// add snapshot to delete slice if past expiration
		if s.SnapshotCreateTime.Before(expirationDate) {
			dbSnapshotsToDelete = append(dbSnapshotsToDelete, s)
			continue
		}
		// if we are still over maxDBSnapshots add to the delete slice
		// skip if maxDBSnapshotsCount is 0
		if i+1 > maxDBSnapshotCount && maxDBSnapshotCount != 0 {
			dbSnapshotsToDelete = append(dbSnapshotsToDelete, s)
		}

	}

	return dbSnapshotsToDelete, nil
}

// findManualDBSnapshots returns a slice of available manual snapshots
func findManualDBSnapshots(client *rds.RDS, dbInstanceIdentifier string) ([]*rds.DBSnapshot, error) {
	var manualDBSnapshots []*rds.DBSnapshot

	input := &rds.DescribeDBSnapshotsInput{
		DBInstanceIdentifier: aws.String(dbInstanceIdentifier),
		IncludePublic:        aws.Bool(false),
		IncludeShared:        aws.Bool(false),
		SnapshotType:         aws.String("manual"),
	}

	res, err := client.DescribeDBSnapshots(input)
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

//deleteDBSnapshot iterates through a list of snapshots and calls deleteDBSnapshot
func deleteDBSnapshots(client *rds.RDS, dbSnapshotsToDelete []*rds.DBSnapshot, dryRun bool) error {
	log.Printf("%d DB snapshots to delete", len(dbSnapshotsToDelete))
	for _, e := range dbSnapshotsToDelete {
		if dryRun {
			log.Printf("Would delete DB snapshot '%v' created on %v", *e.DBSnapshotIdentifier, e.SnapshotCreateTime.Format(RFC8601))
		} else {
			log.Printf("Deleting Snapshot '%v' created on %v", *e.DBSnapshotIdentifier, e.SnapshotCreateTime.Format(RFC8601))
			err := deleteDBSnapshot(client, *e.DBSnapshotIdentifier)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// deleteDBSnapshot deletes DB snapshot and waits for it to complete
func deleteDBSnapshot(client *rds.RDS, DBSnapshotIdentifier string) error {
	deleteDBSnapshotInput := &rds.DeleteDBSnapshotInput{
		DBSnapshotIdentifier: aws.String(DBSnapshotIdentifier),
	}
	_, err := client.DeleteDBSnapshot(deleteDBSnapshotInput)
	if err != nil {
		return err
	}

	WaitUntilDBSnapshotDeletedInput := &rds.DescribeDBSnapshotsInput{
		DBSnapshotIdentifier: aws.String(DBSnapshotIdentifier),
	}
	err = client.WaitUntilDBSnapshotDeleted(WaitUntilDBSnapshotDeletedInput)
	return err
}
