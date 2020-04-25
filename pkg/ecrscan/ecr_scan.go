package ecrscan

import (
	"errors"
	"time"

	"github.com/avast/retry-go"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"go.uber.org/zap"
)

type Target struct {
	Repository string `json:"repository"`
	ImageTag   string `json:"imageTag"`
}

type Report struct {
	Score         string `json:"score"`
	TotalFindings int    `json:"totalFindings"`
}

type Evaluator struct {
	MaxScanAge int
	Logger     *zap.Logger
	ECRClient  ecriface.ECRAPI
}

func (e *Evaluator) Scan(target *Target) {
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

func (e *Evaluator) Evaluate(target *Target) (*Report, error) {
	e.Logger.Info("Evaluating image",
		zap.String("repository", target.Repository),
		zap.String("imageTag", target.ImageTag))
	findings, err := e.GetImageFindings(target)
	if err != nil {
		return nil, err
	}
	if e.IsOldScan(findings.ImageScanFindings) {
		e.Logger.Info("Most recent scan exceeds max scan age")
		e.Scan(target)
		findings, err = e.GetImageFindings(target)
		if err != nil {
			return nil, err
		}
	} else {
		e.Logger.Info("Generating scan report")
	}
	return e.GenerateReport(findings.ImageScanFindings), nil
}

func (e *Evaluator) GetImageFindings(target *Target) (*ecr.DescribeImageScanFindingsOutput, error) {
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
					e.Scan(target)
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

func (e *Evaluator) GenerateReport(findings *ecr.ImageScanFindings) *Report {
	totalFindings := e.CalculateTotalFindings(findings)
	var score string
	if totalFindings == 0 {
		score = "PASS"
	} else {
		score = "FAIL"
	}
	return &Report{
		TotalFindings: totalFindings,
		Score:         score,
	}
}

func (e *Evaluator) IsOldScan(findings *ecr.ImageScanFindings) bool {
	scanTime := findings.ImageScanCompletedAt
	return time.Since(*scanTime).Hours() > float64(e.MaxScanAge)
}

func (e *Evaluator) CalculateTotalFindings(findings *ecr.ImageScanFindings) int {
	counts := findings.FindingSeverityCounts
	total := 0
	for _, v := range counts {
		total += int(*v)
	}
	return total
}
