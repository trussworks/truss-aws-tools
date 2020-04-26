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

type Target struct {
	Repository string `json:"repository" validate:"required"`
	ImageTag   string `json:"imageTag" validate:"required"`
}

type Report struct {
	TotalFindings int `json:"totalFindings"`
}

type Evaluator struct {
	MaxScanAge int
	Logger     *zap.Logger
	ECRClient  ecriface.ECRAPI
}

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
		e.scan(target)
		findings, err = e.getImageFindings(target)
		if err != nil {
			return nil, err
		}
	} else {
		e.Logger.Info("Generating scan report")
	}
	return e.generateReport(findings.ImageScanFindings), nil
}

func (e *Evaluator) scan(target *Target) {
	e.Logger.Info("Scanning image")
	_, err := e.ECRClient.StartImageScan(&ecr.StartImageScanInput{
		ImageId: &ecr.ImageIdentifier{
			ImageTag: aws.String(target.ImageTag),
		},
		RepositoryName: aws.String(target.Repository),
	})
	if err != nil {
		e.Logger.Fatal("Unable to start image scan",
			zap.String("error", err.Error()))
	}
}

func (e *Evaluator) getImageFindings(target *Target) (*ecr.DescribeImageScanFindingsOutput, error) {
	var scanFindings *ecr.DescribeImageScanFindingsOutput
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
				var aerr *ecr.ScanNotFoundException
				if errors.As(err, &aerr) {
					e.Logger.Info("No scan found for image")
					e.scan(target)
					return errors.New("Waiting for new scan to complete")
				} else {
					return retry.Unrecoverable(errors.New("Unable to describe scan findings"))
				}
			}
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

func (e *Evaluator) generateReport(findings *ecr.ImageScanFindings) *Report {
	return &Report{
		TotalFindings: e.calculateTotalFindings(findings),
	}
}

func (e *Evaluator) isOldScan(findings *ecr.ImageScanFindings) bool {
	scanTime := findings.ImageScanCompletedAt
	return time.Since(*scanTime).Hours() > float64(e.MaxScanAge)
}

func (e *Evaluator) calculateTotalFindings(findings *ecr.ImageScanFindings) int {
	counts := findings.FindingSeverityCounts
	total := 0
	for _, v := range counts {
		total += int(*v)
	}
	return total
}
