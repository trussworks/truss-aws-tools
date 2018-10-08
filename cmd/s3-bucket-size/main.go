package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/trussworks/truss-aws-tools/internal/aws/session"
)

func main() {
	var bucket, profile, region string
	flag.StringVar(&bucket, "bucket", "", "The S3 bucket to get the size.")
	flag.StringVar(&region, "region", "", "The AWS region to use.")
	flag.StringVar(&profile, "profile", "", "The AWS profile to use.")
	flag.Parse()
	if bucket == "" {
		flag.PrintDefaults()
		return
	}
	s3Client := makeS3Client(region, profile)
	bucketregion, err := getBucketRegion(s3Client, bucket)
	if err != nil {
		log.Fatal(err)
	}

	cloudWatchClient := makeCloudWatchClient(bucketregion, profile)
	size, err := getBucketSize(cloudWatchClient, bucket)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(size)
}

// makeS3Client makes an S3 client
func makeS3Client(region, profile string) *s3.S3 {
	sess := session.MustMakeSession(region, "")
	c := s3.New(sess)
	return c
}

// makeCloudWatchClient makes a CloudWatch client
func makeCloudWatchClient(region, profile string) *cloudwatch.CloudWatch {
	sess := session.MustMakeSession(region, profile)
	cloudWatchClient := cloudwatch.New(sess)
	return cloudWatchClient
}

func getBucketRegion(c *s3.S3, bucket string) (string, error) {
	res, err := c.GetBucketLocation(&s3.GetBucketLocationInput{
		Bucket: &bucket,
	})
	if err != nil {
		return "", err
	}
	// Ugh. The S3 API returns inconsistent responses; namely, us-east-1
	// and eu-west-1 return non-standard values that need to be converted
	// into the standard region codes.
	// https://docs.aws.amazon.com/AmazonS3/latest/API/RESTBucketGETlocation.html
	var region string
	if res.LocationConstraint != nil {
		region = *res.LocationConstraint
	} else {
		region = ""
	}
	switch region {
	case "":
		return "us-east-1", nil
	case "EU":
		return "eu-west-1", nil
	default:
		return region, nil
	}
}

func getBucketSize(c *cloudwatch.CloudWatch, bucket string) (int, error) {
	var storageTypes = []cloudWatchStorageType{
		standardStorage,
		standardIAStorage,
		reducedRedundancyStorage,
		glacierS3ObjectOverhead,
		standardIAObjectOverhead,
		oneZoneIAStorage,
		oneZoneIAObjectOverhead,
		glacierStorage,
		glacierObjectOverhead,
	}
	size := 0
	for _, storageType := range storageTypes {
		s, err := getBucketSizeInBytes(c, bucket, storageType)
		if err != nil {
			return 0, err
		}
		size += s
	}
	return size, nil
}

func getBucketSizeInBytes(c *cloudwatch.CloudWatch,
	bucket string,
	storageType cloudWatchStorageType) (int, error) {
	i := makeGetMetricStatisticsInputForSize(bucket, storageType)
	m, err := c.GetMetricStatistics(i)
	if err != nil {
		return 0, err
	}
	if m.Datapoints != nil {
		return int(getAverageFromLatestDatapoint(m.Datapoints)), nil
	}
	// If there aren't any objects of a given storage type, there will be
	// no Datapoints. In this case, return 0 bytes.
	return 0, nil
}

func getAverageFromLatestDatapoint(d []*cloudwatch.Datapoint) float64 {
	sort.SliceStable(d, func(i, j int) bool {
		return d[i].Timestamp.Before(*d[j].Timestamp)
	})
	return *d[0].Average
}

type cloudWatchStorageType int

const (
	standardStorage cloudWatchStorageType = iota
	standardIAStorage
	reducedRedundancyStorage
	glacierS3ObjectOverhead
	standardIAObjectOverhead
	oneZoneIAStorage
	oneZoneIAObjectOverhead
	glacierStorage
	glacierObjectOverhead
)

func makeGetMetricStatisticsInputForSize(bucket string, storageType cloudWatchStorageType) *cloudwatch.GetMetricStatisticsInput {
	var storageTypeString string
	switch storageType {
	case standardStorage:
		storageTypeString = "StandardStorage"
	case standardIAStorage:
		storageTypeString = "StandardIAStorage"
	case reducedRedundancyStorage:
		storageTypeString = "ReducedRedundancyStorage"
	case glacierS3ObjectOverhead:
		storageTypeString = "GlacierS3ObjectOverhead"
	case standardIAObjectOverhead:
		storageTypeString = "StandardIAObjectOverhead"
	case oneZoneIAStorage:
		storageTypeString = "OneZoneIAStorage"
	case oneZoneIAObjectOverhead:
		storageTypeString = "OneZoneIAObjectOverhead"
	case glacierStorage:
		storageTypeString = "GlacierStorage"
	case glacierObjectOverhead:
		storageTypeString = "GlacierObjectOverhead"
	}
	now := time.Now()
	// CloudWatch daily metrics can take some time to
	// generate. Even if all of your metrics are generated for
	// "daily at 1AM UTC", you may not be able to see that
	// datapoint for several hours after 1AM UTC while it's been
	// calculated by S3. This means that if you naively ask for
	// metrics since 1 day ago, you'll sometimes get nothing back
	// depending on what time of day you ask. Instead, ask for
	// metrics going back 2 days, and use the latest Datapoint you
	// get.
	startTime := now.Add(-time.Duration(86400*3) * time.Second)
	d := []*cloudwatch.Dimension{
		&cloudwatch.Dimension{
			Name:  aws.String("BucketName"),
			Value: &bucket,
		},
		&cloudwatch.Dimension{
			Name:  aws.String("StorageType"),
			Value: &storageTypeString,
		},
	}
	i := &cloudwatch.GetMetricStatisticsInput{
		Dimensions: d,
		EndTime:    &now,
		MetricName: aws.String("BucketSizeBytes"),
		Namespace:  aws.String("AWS/S3"),
		Period:     aws.Int64(86400),
		StartTime:  &startTime,
		Statistics: []*string{aws.String("Average")},
	}
	return i
}
