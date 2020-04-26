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
	if _, ok := testFindingsOutput[*input.ImageId.ImageTag]; ok {
		return testFindingsOutput[*input.ImageId.ImageTag], nil
	} else {
		return nil, errors.New("error")
	}
}

var testFindingsOutput = map[string]*ecr.DescribeImageScanFindingsOutput{
	"test100": &ecr.DescribeImageScanFindingsOutput{
		ImageScanStatus: &ecr.ImageScanStatus{
			Status: aws.String(ecr.ScanStatusComplete),
		},
	},
}

var testFindings = []struct {
	description    string
	findings       *ecr.ImageScanFindings
	expectedReport Report
	expectedTotal  int
}{
	{
		"Scan with no findings",
		&ecr.ImageScanFindings{
			FindingSeverityCounts: map[string]*int64{
				ecr.FindingSeverityUndefined:     aws.Int64(0),
				ecr.FindingSeverityInformational: aws.Int64(0),
				ecr.FindingSeverityLow:           aws.Int64(0),
				ecr.FindingSeverityMedium:        aws.Int64(0),
				ecr.FindingSeverityHigh:          aws.Int64(0),
				ecr.FindingSeverityCritical:      aws.Int64(0),
			},
		},
		Report{
			TotalFindings: 0,
			Score:         "PASS",
		},
		0,
	},
	{
		"Scan with 1 undefined finding",
		&ecr.ImageScanFindings{
			FindingSeverityCounts: map[string]*int64{
				ecr.FindingSeverityUndefined:     aws.Int64(1),
				ecr.FindingSeverityInformational: aws.Int64(0),
				ecr.FindingSeverityLow:           aws.Int64(0),
				ecr.FindingSeverityMedium:        aws.Int64(0),
				ecr.FindingSeverityHigh:          aws.Int64(0),
				ecr.FindingSeverityCritical:      aws.Int64(0),
			},
		},
		Report{
			TotalFindings: 1,
			Score:         "FAIL",
		},
		1,
	},
	{
		"Scan with 1 critical finding",
		&ecr.ImageScanFindings{

			FindingSeverityCounts: map[string]*int64{
				ecr.FindingSeverityUndefined:     aws.Int64(0),
				ecr.FindingSeverityInformational: aws.Int64(0),
				ecr.FindingSeverityLow:           aws.Int64(0),
				ecr.FindingSeverityMedium:        aws.Int64(0),
				ecr.FindingSeverityHigh:          aws.Int64(0),
				ecr.FindingSeverityCritical:      aws.Int64(1),
			},
		},
		Report{
			TotalFindings: 1,
			Score:         "FAIL",
		},
		1,
	},
	{
		"Scan with 1 finding in each category",
		&ecr.ImageScanFindings{
			FindingSeverityCounts: map[string]*int64{
				ecr.FindingSeverityUndefined:     aws.Int64(1),
				ecr.FindingSeverityInformational: aws.Int64(1),
				ecr.FindingSeverityLow:           aws.Int64(1),
				ecr.FindingSeverityMedium:        aws.Int64(1),
				ecr.FindingSeverityHigh:          aws.Int64(1),
				ecr.FindingSeverityCritical:      aws.Int64(1),
			},
		},
		Report{
			TotalFindings: 6,
			Score:         "FAIL",
		},
		6,
	},
	{
		"Scan with multiple findings in each category",
		&ecr.ImageScanFindings{
			FindingSeverityCounts: map[string]*int64{
				ecr.FindingSeverityUndefined:     aws.Int64(5),
				ecr.FindingSeverityInformational: aws.Int64(8),
				ecr.FindingSeverityLow:           aws.Int64(13),
				ecr.FindingSeverityMedium:        aws.Int64(21),
				ecr.FindingSeverityHigh:          aws.Int64(34),
				ecr.FindingSeverityCritical:      aws.Int64(55),
			},
		},
		Report{
			TotalFindings: 136,
			Score:         "FAIL",
		},
		136,
	},
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
	for _, tt := range testFindings {
		testname := tt.description
		t.Run(testname, func(t *testing.T) {
			total := evaluator.calculateTotalFindings(tt.findings)
			if total != tt.expectedTotal {
				t.Errorf("got %d, want %d", total, tt.expectedTotal)
			}
		})
	}
}

func TestGenerateReport(t *testing.T) {
	for _, tt := range testFindings {
		testname := tt.description
		t.Run(testname, func(t *testing.T) {
			report := evaluator.generateReport(tt.findings)
			if !cmp.Equal(*report, tt.expectedReport) {
				t.Errorf("got %+v, want %+v", *report, tt.expectedReport)
			}
		})
	}
}

func TestGetImageFindings(t *testing.T) {
	tests := []struct {
		description string
		target      *Target
		expected    *ecr.DescribeImageScanFindingsOutput
	}{
		{
			"Nonexistent image",
			&Target{
				ImageTag:   "test123",
				Repository: "testrepo",
			},
			nil,
		},
		{
			"Existing image",
			&Target{
				ImageTag:   "test100",
				Repository: "testrepo",
			},
			testFindingsOutput["test100"],
		},
	}
	for _, tt := range tests {
		testname := tt.description
		t.Run(testname, func(t *testing.T) {
			findings, _ := evaluator.getImageFindings(tt.target)
			if findings != tt.expected {
				t.Errorf("got %+v, want %+v", *findings, *tt.expected)
			}
		})
	}
}
