package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {
	var bucket, region string
	flag.StringVar(&bucket, "bucket", "", "The S3 bucket to get the size.")
	flag.StringVar(&region, "region", "us-east-1", "The AWS region to use.")
	flag.Parse()
	if bucket == "" {
		flag.PrintDefaults()
		return
	}
	s3Client, err := makeS3Client(region)
	if err != nil {
		log.Fatal(err)
	}
	bucketregion, err := getBucketRegion(s3Client, bucket)
	if err != nil {
		log.Fatal(err)
	}

	cloudWatchClient, err := makeCloudWatchClient(bucketregion)
	if err != nil {
		log.Fatal(err)

	}
	size, err := getBucketSize(cloudWatchClient, bucket)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(size)
}

// makeS3Client makes an S3 client
func makeS3Client(region string) (*s3.S3, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: &region,
	})
	if err != nil {
		return nil, err
	}
	c := s3.New(sess)
	return c, nil
}

// makeCloudWatchClient makes a CloudWatch client
func makeCloudWatchClient(region string) (*cloudwatch.CloudWatch, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: &region,
	})
	if err != nil {
		return nil, err
	}
	cloudWatchClient := cloudwatch.New(sess)
	return cloudWatchClient, nil
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
		return int(*m.Datapoints[0].Average), nil
	}
	// If there aren't any objects of a given storage type, there will be
	// no Datapoints. In this case, return 0 bytes.
	return 0, nil
}

type cloudWatchStorageType int

const (
	standardStorage cloudWatchStorageType = iota
	standardIAStorage
	reducedRedundancyStorage
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
	}
	now := time.Now()
	startTime := now.Add(-time.Duration(86400) * time.Second)
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
