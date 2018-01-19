package session

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
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

// MustMakeSession creates an AWS Session using MakeSession and ensures
// that it is valid.
func MustMakeSession(region, profile string) *session.Session {
	return session.Must(MakeSession(region, profile))
}
