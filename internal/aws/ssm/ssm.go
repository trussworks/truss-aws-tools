package ssm

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/pkg/errors"
)

// DecryptValue returns the decrypted value for a Parameter Store key
func DecryptValue(session *session.Session, parameterStoreKey string) (string, error) {
	ssmClient := ssm.New(session)
	getParameterOutput, err := ssmClient.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(parameterStoreKey),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == ssm.ErrCodeInternalServerError {
				return "", errors.Wrap(err, "ssm appeared to have an internal error")
			} else if aerr.Code() == ssm.ErrCodeInvalidKeyId || aerr.Code() == ssm.ErrCodeParameterNotFound || aerr.Code() == ssm.ErrCodeParameterVersionNotFound {
				return "", errors.Wrap(err, "the provided patameter store key appears to be invalid")
			} else {
				return "", errors.Wrap(err, "unknown AWS error")
			}
		}
		return "", errors.Wrap(err, "unknown error getting ssm parameter")
	}
	if getParameterOutput.Parameter.Value == nil {
		return "", errors.Wrap(err, "ssm parameter value is nil")
	}
	return *getParameterOutput.Parameter.Value, nil
}
