package rdsclean

import (
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"go.uber.org/zap"
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

	logger, _ := zap.NewProduction()
	r := RDSManualSnapshotClean{
		DBInstanceIdentifier: "cleanme",
		DryRun:               true,
		ExpirationDate:       getTime("2017-03-02T22:00:00+00:00"),
		Logger:               logger,
		MaxDBSnapshotCount:   0,
		RDSClient:            nil,
	}

	//expirationTime := getTime("2017-03-02T22:00:00+00:00")
	//maxDBSnapshotCount := 0
	wantExpiredDBSnapshots := []*rds.DBSnapshot{oldDBSnapshot}

	haveExpiredDBSnapshots, err := r.FindDBSnapshotsToDelete(dbSnapshots)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wantExpiredDBSnapshots, haveExpiredDBSnapshots) {
		t.Fatalf("FindDBSnapshotsToDelete(haveDBSnapshots) = %v, \nwant = %v",
			haveExpiredDBSnapshots,
			wantExpiredDBSnapshots)
	}

	r.ExpirationDate = getTime("2017-02-28T22:00:00+00:00")
	r.MaxDBSnapshotCount = 2
	wantMaxDBSnapshots := []*rds.DBSnapshot{oldDBSnapshot}
	haveMaxDBSnapshots, err := r.FindDBSnapshotsToDelete(dbSnapshots)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wantMaxDBSnapshots, haveMaxDBSnapshots) {
		t.Fatalf("FindDBSnapshotsToDelete(haveDBSnapshots) = %v, \nwant = %v",
			haveMaxDBSnapshots,
			wantMaxDBSnapshots)
	}

}
