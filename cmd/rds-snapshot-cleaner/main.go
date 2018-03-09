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
	var retentionDays, maxSnapshotCount int
	dryRun := false

	flag.StringVar(&dbInstanceIdentifier, "db-identifier",
		"",
		"The RDS instance identifier.")
	flag.IntVar(&retentionDays, "retention-days",
		30,
		"The maximum retention age in days.")
	flag.IntVar(&maxSnapshotCount, "max-snapshots",
		50,
		"The maximum number of manual snapshots allowed. This takes precedence over -retention-days.")
	flag.StringVar(&region, "region", "", "The AWS region to use.")
	flag.StringVar(&profile, "profile", "", "The AWS profile to use.")
	flag.BoolVar(&dryRun, "dry-run", false,
		"Don't make any changes and log what would have happened.")
	flag.Parse()

	if dbInstanceIdentifier == "" {
		log.Fatal("DB instance identifier is required")
	}

	if maxSnapshotCount < 0 {
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

	// first look for snapshots that are expired
	expiredDBSnapshots, err := findExpiredDBSnapshots(manualDBSnapshots, expirationDate)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Deleting expired DB snapshots")
	err = deleteDBSnapshots(rdsClient, expiredDBSnapshots, dryRun)
	if err != nil {
		log.Fatal(err)
	}

	// then look for snapshots that are beyond the max snapshot count
	manualDBSnapshots, err = findManualDBSnapshots(rdsClient, dbInstanceIdentifier)
	if err != nil {
		log.Fatal(err)
	}
	overProvisionedDBSnapshots := findOverProvisionedDBSnapshots(manualDBSnapshots, maxSnapshotCount)
	log.Println("Deleting over provisioned DB snapshots")
	err = deleteDBSnapshots(rdsClient, overProvisionedDBSnapshots, dryRun)
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

// sortDBSnapshots sorts a slice of DB snapshots in reverse chronological order(oldest first) using SnapshotCreateTime
func sortDBSnapshots(dbSnapshots []*rds.DBSnapshot) {
	// sort by snapshot creation time
	sort.Slice(dbSnapshots, func(i, j int) bool {
		return dbSnapshots[i].SnapshotCreateTime.Before(*dbSnapshots[j].SnapshotCreateTime)
	})
}

// findExpiredDBSnapshots iterates through a slice of db snapshots and deletes the ones that are expired
func findExpiredDBSnapshots(dbSnapshots []*rds.DBSnapshot, expirationDate time.Time) ([]*rds.DBSnapshot, error) {
	var expiredDBSnapshots []*rds.DBSnapshot

	sortDBSnapshots(dbSnapshots)
	for _, s := range dbSnapshots {
		if s.SnapshotCreateTime.After(expirationDate) {
			break
		}

		expiredDBSnapshots = append(expiredDBSnapshots, s)
	}

	return expiredDBSnapshots, nil
}

// findOverProvisionedDBSnapshots will return a list of snapshots that are beyond the maxSnapshotCount
func findOverProvisionedDBSnapshots(dbSnapshots []*rds.DBSnapshot, maxSnapshotCount int) []*rds.DBSnapshot {
	sortDBSnapshots(dbSnapshots)
	if maxSnapshotCount <= len(dbSnapshots) {
		overProvisionedDBSnapshots := dbSnapshots[:len(dbSnapshots)-maxSnapshotCount]
		return overProvisionedDBSnapshots
	}
	return nil

}

//deleteDBSnapshot iterates through a list of snapshots and calls deleteDBSnapshot
func deleteDBSnapshots(client *rds.RDS, expiredDBSnapshots []*rds.DBSnapshot, dryRun bool) error {
	for _, e := range expiredDBSnapshots {
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
