package ecrscan

import (
	"errors"
	"time"

	"github.com/avast/retry-go"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

var validate *validator.Validate

// Target specifies the ECR image to retrieve scan findings for
type Target struct {
	Repository string `json:"repository" validate:"required"`
	ImageTag   string `json:"imageTag" validate:"required"`
}

// Report contains scan finding information to be returned to the caller
type Report struct {
	TotalFindings int `json:"totalFindings"`
}

// Evaluator orchestrates the retrieval and analysis of image scan findings
type Evaluator struct {
	MaxScanAge int
	Logger     *zap.Logger
	ECRClient  ecriface.ECRAPI
}

// Evaluate returns vulnerability scan information for a specified ECR image.
// The function queries ECR for the image's current scan findings. If the most
// recent scan is older than MaxScanAge, the function initiates a new scan. The
// function response includes a Report of the image's total scan findings.
func (e *Evaluator) Evaluate(target *Target) (*Report, error) {
	validate = validator.New()
	err := validate.Struct(target)
	if err != nil {
		e.Logger.Error("Invalid input", zap.Error(err))
		return nil, errors.New("Invalid target")
	}
	e.Logger.Info("Evaluating image",
		zap.String("repository", target.Repository),
		zap.String("imageTag", target.ImageTag))
	findings, err := e.getImageFindings(target)
	if err != nil {
		return nil, err
	}
	if e.isOldScan(findings.ImageScanFindings) {
		e.Logger.Info("Most recent scan exceeds max scan age")
		if scanErr := e.scan(target); scanErr != nil {
			return nil, scanErr
		}
		findings, err = e.getImageFindings(target)
		if err != nil {
			return nil, err
		}
	} else {
		e.Logger.Info("Generating scan report")
	}
	return &Report{
		TotalFindings: e.calculateTotalFindings(findings.ImageScanFindings),
	}, nil
}

// scan initiates an ECR vulnerability scan for an image.
func (e *Evaluator) scan(target *Target) error {
	e.Logger.Info("Scanning image")
	_, err := e.ECRClient.StartImageScan(&ecr.StartImageScanInput{
		ImageId: &ecr.ImageIdentifier{
			ImageTag: aws.String(target.ImageTag),
		},
		RepositoryName: aws.String(target.Repository),
	})
	if err != nil {
		e.Logger.Error("Unable to start image scan",
			zap.String("error", err.Error()))
	}
	return err
}

// getImageFindings returns image scan findings for a target image. It will wait
// until an image scan is complete and will initiate a scan if an existing scan
// is not found.
func (e *Evaluator) getImageFindings(target *Target) (*ecr.DescribeImageScanFindingsOutput, error) {
	var scanFindings *ecr.DescribeImageScanFindingsOutput
	// Wrap the query in a retry
	err := retry.Do(
		func() error {
			result, err := e.ECRClient.DescribeImageScanFindings(
				&ecr.DescribeImageScanFindingsInput{
					ImageId: &ecr.ImageIdentifier{
						ImageTag: aws.String(target.ImageTag),
					},
					RepositoryName: aws.String(target.Repository),
				})
			if err != nil {
				// If the repo is not configured to scan on
				// push, the image may not have an existing
				// scan. In that scenario, initiate a scan.
				var aerr *ecr.ScanNotFoundException
				if errors.As(err, &aerr) {
					e.Logger.Info("No scan found for image")
					if scanErr := e.scan(target); scanErr != nil {
						return retry.Unrecoverable(errors.New("Error scanning image"))
					}
					return errors.New("Waiting for new scan to complete")
				} else {
					return retry.Unrecoverable(errors.New("Unable to describe scan findings"))
				}
			}
			// Check the scan status and drop out of the retry block
			// if the scan is complete.
			switch scanStatus := *result.ImageScanStatus.Status; scanStatus {
			case ecr.ScanStatusFailed:
				return retry.Unrecoverable(errors.New("Image scan failed"))
			case ecr.ScanStatusInProgress:
				return errors.New("Image scan still in progress")
			case ecr.ScanStatusComplete:
				scanFindings = result
			}
			return nil
		},
		retry.OnRetry(func(n uint, err error) {
			e.Logger.Info("Retry describe image scan findings",
				zap.Int("attempt", int(n)),
				zap.String("reason", err.Error()))
		}),
		retry.Delay(time.Duration(15)*time.Second),
	)
	if err != nil {
		return nil, errors.New("Unable to retrieve scan findings")
	}
	return scanFindings, nil
}

// isOldScan returns true if the image scan was completed more than MaxScanAge
// hours ago relative to the current time; false otherwise.
func (e *Evaluator) isOldScan(findings *ecr.ImageScanFindings) bool {
	scanTime := findings.ImageScanCompletedAt
	return time.Since(*scanTime).Hours() > float64(e.MaxScanAge)
}

// calculateTotalFindings returns the number of findings in the image scan. By
// default, the call to DescribeImageScanFindings
// (https://docs.aws.amazon.com/sdk-for-go/api/service/ecr/#ECR.DescribeImageScanFindings)
// returns a maximum of 100 results unless the MaxResults parameter is specified
// (https://docs.aws.amazon.com/sdk-for-go/api/service/ecr/#DescribeImageScanFindingsInput).
// Thus, rather than relying on the length of the ImageScanFinding slice, the
// function calculates total findings based on the FindingSeverityCounts map.
func (e *Evaluator) calculateTotalFindings(findings *ecr.ImageScanFindings) int {
	counts := findings.FindingSeverityCounts
	total := 0
	for _, v := range counts {
		total += int(*v)
	}
	return total
}
