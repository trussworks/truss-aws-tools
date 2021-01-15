package session

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sts"
)

// MakeSession creates an AWS Session, with appropriate defaults,
// using shared credentials, and with region and profile overrides.
func MakeSession(region, profile string) (*session.Session, error) {
	sessOpts := session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}
	if profile != "" {
		sessOpts.Profile = profile
	}
	if region != "" {
		sessOpts.Config = aws.Config{
			Region: aws.String(region),
		}
	}
	return session.NewSessionWithOptions(sessOpts)
}

// MakeSessionWithSTSCredentials creates an AWS Session, with appropriate defaults,
// using custom AWS STS credentials
func MakeSessionWithSTSCredentials(stsCredentials *sts.Credentials) (*session.Session, error) {
	awsConfig := aws.Config{
		Credentials: credentials.NewStaticCredentials(*stsCredentials.AccessKeyId, *stsCredentials.SecretAccessKey, *stsCredentials.SessionToken),
		Region:      aws.String("us-east-1"),
	}
	sessOpts := session.Options{
		Config: awsConfig,
	}
	return session.NewSessionWithOptions(sessOpts)
}

// MustMakeSessionWithSTSCredentials creates an AWS Session using MakeSessionWithSTSCredentials and ensures
// that it is valid.
func MustMakeSessionWithSTSCredentials(credentials *sts.Credentials) *session.Session {
	return session.Must(MakeSessionWithSTSCredentials(credentials))
}

// MustMakeSession creates an AWS Session using MakeSession and ensures
// that it is valid.
func MustMakeSession(region, profile string) *session.Session {
	return session.Must(MakeSession(region, profile))
}

// MakeEC2Client makes an AWS EC2 client from an AWS Session.
func MakeEC2Client(session *session.Session) *ec2.EC2 {
	return ec2.New(session)
}

// MakeSTSClient makes an AWS STS client from an AWS Session.
func MakeSTSClient(session *session.Session) *sts.STS {
	return sts.New(session)
}
