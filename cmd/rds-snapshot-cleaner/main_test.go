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
	SnapshotCreateTime:   aws.Time(getTime("2017-03-03T22:00:00+00:00")),
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
		newDBSnapshot,
		oldDBSnapshot}
	haveDBSnapshots := []*rds.DBSnapshot{
		oldDBSnapshot,
		newDBSnapshot}

	sortDBSnapshots(haveDBSnapshots)
	if !reflect.DeepEqual(wantDBSnapshots, haveDBSnapshots) {
		t.Fatalf("sortDBSnapshots(haveDBSnapshots) = %v, \nwant = %v",
			haveDBSnapshots,
			wantDBSnapshots)
	}

}

func TestFindDBSnapshotsToDelete(t *testing.T) {
	dbSnapshots := []*rds.DBSnapshot{
		newDBSnapshot,
		newDBSnapshot,
		oldDBSnapshot,
	}
	expirationTime := getTime("2017-03-02T22:00:00+00:00")
	maxDBSnapshotCount := 0
	wantExpiredDBSnapshots := []*rds.DBSnapshot{oldDBSnapshot}

	haveExpiredDBSnapshots, err := findDBSnapshotsToDelete(dbSnapshots, expirationTime, maxDBSnapshotCount)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wantExpiredDBSnapshots, haveExpiredDBSnapshots) {
		t.Fatalf("findDBSnapshotsToDelete(haveDBSnapshots, %s, %d) = %v, \nwant = %v",
			expirationTime,
			maxDBSnapshotCount,
			haveExpiredDBSnapshots,
			wantExpiredDBSnapshots)
	}

	expirationTime = getTime("2017-02-28T22:00:00+00:00")
	wantMaxDBSnapshots := []*rds.DBSnapshot{oldDBSnapshot}
	haveMaxDBSnapshots, err := findDBSnapshotsToDelete(dbSnapshots, expirationTime, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wantMaxDBSnapshots, haveMaxDBSnapshots) {
		t.Fatalf("findDBSnapshotsToDelete(haveDBSnapshots, %s, %d) = %v, \nwant = %v",
			expirationTime,
			maxDBSnapshotCount,
			haveMaxDBSnapshots,
			wantMaxDBSnapshots)
	}

}
