package tarefresh

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/support"
	"go.uber.org/zap"
)

// TrustedAdvisorRefresh is a AWS support session for refreshing Trusted Advisor
type TrustedAdvisorRefresh struct {
	Logger        *zap.Logger
	SupportClient *support.Support
}

func isCheckRefreshable(check string) bool {
	unrefreshableChecks := []string{
		"AWS Direct Connect Connection Redundancy",
		"AWS Direct Connect Location Redundancy",
		"AWS Direct Connect Virtual Interface Redundancy",
		"PV Driver Version for EC2 Windows Instances",
		"EC2Config Service for EC2 Windows Instances",
		"Amazon EBS Public Snapshots",
		"Amazon RDS Public Snapshots",
	}

	for _, u := range unrefreshableChecks {
		if u == check {
			return false
		}
	}
	return true
}

// Refresh iterates through all Trusted Advisor checks and triggers a refresh.
// Unrefreshable checks will be logged as an error
func (r *TrustedAdvisorRefresh) Refresh() error {
	describeParams := &support.DescribeTrustedAdvisorChecksInput{
		Language: aws.String("en"),
	}
	resp, err := r.SupportClient.DescribeTrustedAdvisorChecks(describeParams)
	if err != nil {
		return err
	}

	for _, s := range resp.Checks {
		checkParams := &support.RefreshTrustedAdvisorCheckInput{
			CheckId: aws.String(*s.Id), // Required
		}

		r.Logger.Info("refreshing",
			zap.String("name", *s.Name),
			zap.String("id", *s.Id),
		)
		if isCheckRefreshable(*s.Name) {
			_, err = r.SupportClient.RefreshTrustedAdvisorCheck(checkParams)
			if err != nil {
				r.Logger.Error("unable to refresh", zap.Error(err))
			}
		}

	}
	return err
}
