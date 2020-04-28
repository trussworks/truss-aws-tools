package ecrscan

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

type mockECRClient struct {
	mock.Mock
	ecriface.ECRAPI
}

var maxScanAge = 24
var logger, _ = zap.NewProduction()
var ecrClient = &mockECRClient{}
var evaluator = Evaluator{
	MaxScanAge: maxScanAge,
	Logger:     logger,
	ECRClient:  ecrClient,
}

func relativeTimePointer(hours float64) *time.Time {
	t := time.Now()
	newT := t.Add(-time.Duration(hours) * time.Hour)
	return &newT
}

func (m *mockECRClient) DescribeImageScanFindings(input *ecr.DescribeImageScanFindingsInput) (*ecr.DescribeImageScanFindingsOutput, error) {
	if _, ok := testCases[*input.ImageId.ImageTag]; ok {
		return testCases[*input.ImageId.ImageTag], nil
	} else {
		return nil, errors.New("error")
	}
}

var testCases = map[string]*ecr.DescribeImageScanFindingsOutput{
	"ScanCompletedNoFindings": &ecr.DescribeImageScanFindingsOutput{
		ImageScanFindings: &ecr.ImageScanFindings{
			FindingSeverityCounts: map[string]*int64{
				ecr.FindingSeverityUndefined:     aws.Int64(0),
				ecr.FindingSeverityInformational: aws.Int64(0),
				ecr.FindingSeverityLow:           aws.Int64(0),
				ecr.FindingSeverityMedium:        aws.Int64(0),
				ecr.FindingSeverityHigh:          aws.Int64(0),
				ecr.FindingSeverityCritical:      aws.Int64(0),
			},
			ImageScanCompletedAt: relativeTimePointer(1),
		},
		ImageScanStatus: &ecr.ImageScanStatus{
			Status: aws.String(ecr.ScanStatusComplete),
		},
	},
	"ScanCompletedOneUndefinedFinding": &ecr.DescribeImageScanFindingsOutput{
		ImageScanFindings: &ecr.ImageScanFindings{
			FindingSeverityCounts: map[string]*int64{
				ecr.FindingSeverityUndefined:     aws.Int64(1),
				ecr.FindingSeverityInformational: aws.Int64(0),
				ecr.FindingSeverityLow:           aws.Int64(0),
				ecr.FindingSeverityMedium:        aws.Int64(0),
				ecr.FindingSeverityHigh:          aws.Int64(0),
				ecr.FindingSeverityCritical:      aws.Int64(0),
			},
			ImageScanCompletedAt: relativeTimePointer(1),
		},
		ImageScanStatus: &ecr.ImageScanStatus{
			Status: aws.String(ecr.ScanStatusComplete),
		},
	},
	"ScanCompletedOneCriticalFinding": &ecr.DescribeImageScanFindingsOutput{
		ImageScanFindings: &ecr.ImageScanFindings{
			FindingSeverityCounts: map[string]*int64{
				ecr.FindingSeverityUndefined:     aws.Int64(0),
				ecr.FindingSeverityInformational: aws.Int64(0),
				ecr.FindingSeverityLow:           aws.Int64(0),
				ecr.FindingSeverityMedium:        aws.Int64(0),
				ecr.FindingSeverityHigh:          aws.Int64(0),
				ecr.FindingSeverityCritical:      aws.Int64(1),
			},
			ImageScanCompletedAt: relativeTimePointer(1),
		},
		ImageScanStatus: &ecr.ImageScanStatus{
			Status: aws.String(ecr.ScanStatusComplete),
		},
	},

	"ScanCompletedOneFindingEachCategory": &ecr.DescribeImageScanFindingsOutput{
		ImageScanFindings: &ecr.ImageScanFindings{
			FindingSeverityCounts: map[string]*int64{
				ecr.FindingSeverityUndefined:     aws.Int64(1),
				ecr.FindingSeverityInformational: aws.Int64(1),
				ecr.FindingSeverityLow:           aws.Int64(1),
				ecr.FindingSeverityMedium:        aws.Int64(1),
				ecr.FindingSeverityHigh:          aws.Int64(1),
				ecr.FindingSeverityCritical:      aws.Int64(1),
			},
			ImageScanCompletedAt: relativeTimePointer(1),
		},
		ImageScanStatus: &ecr.ImageScanStatus{
			Status: aws.String(ecr.ScanStatusComplete),
		},
	},
	"ScanCompletedMultipleFindingsEachCategory": &ecr.DescribeImageScanFindingsOutput{
		ImageScanFindings: &ecr.ImageScanFindings{
			FindingSeverityCounts: map[string]*int64{
				ecr.FindingSeverityUndefined:     aws.Int64(5),
				ecr.FindingSeverityInformational: aws.Int64(8),
				ecr.FindingSeverityLow:           aws.Int64(13),
				ecr.FindingSeverityMedium:        aws.Int64(21),
				ecr.FindingSeverityHigh:          aws.Int64(34),
				ecr.FindingSeverityCritical:      aws.Int64(55),
			},
			ImageScanCompletedAt: relativeTimePointer(1),
		},
		ImageScanStatus: &ecr.ImageScanStatus{
			Status: aws.String(ecr.ScanStatusComplete),
		},
	},
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		target   *Target
		expected *Report
	}{
		{
			&Target{
				ImageTag:   "ScanCompletedNoFindings",
				Repository: "test-repo",
			},
			&Report{
				TotalFindings: 0,
			},
		},
		{
			&Target{
				ImageTag:   "ScanCompletedOneUndefinedFinding",
				Repository: "test-repo",
			},
			&Report{
				TotalFindings: 1,
			},
		},
		{
			&Target{
				ImageTag:   "ScanCompletedOneCriticalFinding",
				Repository: "test-repo",
			},
			&Report{
				TotalFindings: 1,
			},
		},
		{
			&Target{
				ImageTag:   "ScanCompletedOneFindingEachCategory",
				Repository: "test-repo",
			},
			&Report{
				TotalFindings: 6,
			},
		},
		{
			&Target{
				ImageTag:   "ScanCompletedMultipleFindingsEachCategory",
				Repository: "test-repo",
			},
			&Report{
				TotalFindings: 136,
			},
		},
	}
	for _, tt := range tests {
		testname := tt.target.ImageTag
		t.Run(testname, func(t *testing.T) {
			report, _ := evaluator.Evaluate(tt.target)
			if !cmp.Equal(report, tt.expected) {
				t.Errorf("got %+v, want %+v", *report, *tt.expected)
			}
		})
	}
}

func TestEvaluateWithBadInput(t *testing.T) {
	tests := []struct {
		description string
		target      *Target
	}{
		{
			"Nil target",
			nil,
		},
		{
			"Empty target",
			&Target{},
		},
		{
			"No repository",
			&Target{
				ImageTag: "test123",
			},
		},
		{
			"No image tag",
			&Target{
				Repository: "testrepo",
			},
		},
	}
	for _, tt := range tests {
		testname := tt.description
		t.Run(testname, func(t *testing.T) {
			report, err := evaluator.Evaluate(tt.target)
			if err == nil {
				t.Errorf("got %+v, want error", *report)
			}
		})
	}
}
func TestIsOldScan(t *testing.T) {
	tests := []struct {
		description string
		findings    *ecr.ImageScanFindings
		expected    bool
	}{
		{
			"Scan created in the future",
			&ecr.ImageScanFindings{
				ImageScanCompletedAt: relativeTimePointer(-3),
			},
			false,
		},
		{
			"Scan created now",
			&ecr.ImageScanFindings{
				ImageScanCompletedAt: relativeTimePointer(0),
			},
			false,
		},
		{
			"Scan created 1 hour ago",
			&ecr.ImageScanFindings{
				ImageScanCompletedAt: relativeTimePointer(1),
			},
			false,
		},
		{
			"Scan created 1 hour before max age",
			&ecr.ImageScanFindings{
				ImageScanCompletedAt: relativeTimePointer(float64(maxScanAge - 1)),
			},
			false,
		},
		{
			"Scan created 1 hour after max age",
			&ecr.ImageScanFindings{
				ImageScanCompletedAt: relativeTimePointer(float64(maxScanAge + 1)),
			},
			true,
		},
		{
			"Scan created fraction before max age",
			&ecr.ImageScanFindings{
				ImageScanCompletedAt: relativeTimePointer(float64(maxScanAge) - 0.0000000000001),
			},
			false,
		},
		{
			"Scan created fraction after max age",
			&ecr.ImageScanFindings{
				ImageScanCompletedAt: relativeTimePointer(float64(maxScanAge) + 0.0000000000001),
			},
			true,
		},
		{
			"Scan created far in the past",
			&ecr.ImageScanFindings{
				ImageScanCompletedAt: relativeTimePointer(float64(maxScanAge + 999999)),
			},
			true,
		},
	}
	for _, tt := range tests {
		testname := tt.description
		t.Run(testname, func(t *testing.T) {
			result := evaluator.isOldScan(tt.findings)
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCalculateTotalFindings(t *testing.T) {
	tests := []struct {
		description string
		expected    int
	}{
		{
			"ScanCompletedNoFindings",
			0,
		},
		{
			"ScanCompletedOneUndefinedFinding",
			1,
		},
		{
			"ScanCompletedOneCriticalFinding",
			1,
		},
		{
			"ScanCompletedOneFindingEachCategory",
			6,
		},
		{
			"ScanCompletedMultipleFindingsEachCategory",
			136,
		},
	}
	for _, tt := range tests {
		testname := tt.description
		t.Run(testname, func(t *testing.T) {
			total := evaluator.calculateTotalFindings(testCases[tt.description].ImageScanFindings)
			if total != tt.expected {
				t.Errorf("got %d, want %d", total, tt.expected)
			}
		})
	}
}
