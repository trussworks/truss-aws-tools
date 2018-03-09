package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
)

var oldDBSnapshot = &rds.DBSnapshot{
	DBInstanceIdentifier: aws.String("foo-db"),
	DBSnapshotIdentifier: aws.String("old-snapshot"),
	SnapshotCreateTime:   aws.Time(getTime("2017-03-01T22:00:00+00:00")),
	Status:               aws.String("available"),
}

var newDBSnapshot = &rds.DBSnapshot{
	DBInstanceIdentifier: aws.String("foo-db"),
	DBSnapshotIdentifier: aws.String("new-snapshot"),
	SnapshotCreateTime:   aws.Time(getTime("2017-03-02T22:00:00+00:00")),
	Status:               aws.String("available"),
}

func getTime(original string) (parsed time.Time) {
	parsed, _ = time.Parse(
		RFC8601,
		original,
	)
	return
}

func TestSortDBSnapshots(t *testing.T) {
	wantDBSnapshots := []*rds.DBSnapshot{
		oldDBSnapshot,
		newDBSnapshot}
	haveDBSnapshots := []*rds.DBSnapshot{
		newDBSnapshot,
		oldDBSnapshot}

	sortDBSnapshots(haveDBSnapshots)
	if !reflect.DeepEqual(wantDBSnapshots, haveDBSnapshots) {
		t.Fatalf("sortDBSnapshots(haveDBSnapshots) = %v, \nwant = %v",
			haveDBSnapshots,
			wantDBSnapshots)
	}

}

func TestFindExpiredDBSnapshots(t *testing.T) {
	dbSnapshots := []*rds.DBSnapshot{
		newDBSnapshot,
		oldDBSnapshot,
	}
	wantExpiredDBSnapshots := []*rds.DBSnapshot{oldDBSnapshot}
	expirationTime := getTime("2017-03-01T22:00:00+00:00")

	haveExpiredDBSnapshots, err := findExpiredDBSnapshots(dbSnapshots, expirationTime)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wantExpiredDBSnapshots, haveExpiredDBSnapshots) {
		t.Fatalf("findExpiredDBSnapshots(haveDBSnapshots, %s) = %v, \nwant = %v",
			expirationTime,
			haveExpiredDBSnapshots,
			wantExpiredDBSnapshots)
	}

}

func TestFindOverProvisionedDBSnapshots(t *testing.T) {
	dbSnapshots := []*rds.DBSnapshot{
		newDBSnapshot,
		oldDBSnapshot,
	}
	wantOverProvisionedDBSnapshots := []*rds.DBSnapshot{oldDBSnapshot}
	haveOverProvisionedDBSnapshots := findOverProvisionedDBSnapshots(dbSnapshots, 1)

	if !reflect.DeepEqual(wantOverProvisionedDBSnapshots, haveOverProvisionedDBSnapshots) {
		t.Fatalf("findOverProvisionedDBSnapshots(haveDBSnapshots, 1) = %v, \nwant = %v",
			haveOverProvisionedDBSnapshots,
			wantOverProvisionedDBSnapshots)
	}
}
