package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"reflect"
	"testing"
)

func newCloudFormedVolume() *ec2.Volume {
	v := &ec2.Volume{
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("aws:cloudformation:stack-name"),
				Value: aws.String("TestStack"),
			},
		},
	}
	return v
}

func newVolume() *ec2.Volume {
	v := &ec2.Volume{}
	return v
}

func newTags() []*ec2.Tag {
	return []*ec2.Tag{
		{
			Key:   aws.String("aws:cloudformation:stack-name"),
			Value: aws.String("TestStack"),
		},
		{
			Key:   aws.String("monkey"),
			Value: aws.String("monkey-value"),
		},
	}
}

func expectedTags() []*ec2.Tag {
	return []*ec2.Tag{
		{
			Key:   aws.String("X-aws:cloudformation:stack-name"),
			Value: aws.String("TestStack"),
		},
		{
			Key:   aws.String("monkey"),
			Value: aws.String("monkey-value"),
		},
	}
}

func TestIsCloudformed(t *testing.T) {
	want := true
	wantStackName := "TestStack"
	have, haveStackName := isCloudFormed(newCloudFormedVolume())
	if have != want {
		t.Fatalf("isCloudFormed(cloudFormedVolume) = %v, want = %v",
			have, want)
	}
	if haveStackName != wantStackName {
		t.Fatalf("have StackName: %v, want %v",
			haveStackName,
			wantStackName)
	}

	want = false
	have, haveStackName = isCloudFormed(newVolume())
	if have != want {
		t.Fatalf("isCloudFormed(volume) = %v, want %v", have, want)
	}
}

func TestCopyTags(t *testing.T) {
	have := copyTags(newTags())
	want := expectedTags()
	if !reflect.DeepEqual(have, want) {
		t.Fatalf("have %v, want %v", have, want)
	}
}
