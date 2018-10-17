package awshealth

import (
	"fmt"
)

// PersonalHealthDashboardURL is the URL to view AWS's Personal Health Dashboard
const PersonalHealthDashboardURL = "https://phd.aws.amazon.com/phd/home"

// EventDescription is AWS Health Event Descripion
type EventDescription struct {
	Language string `json:"language"`
	Latest   string `json:"latestDescription"`
}

// Event defines relevant info out of the json payload from AWS Personal Health
type Event struct {
	Description       []EventDescription `json:"eventDescription"`
	EventARN          string             `json:"eventArn"`
	EventTypeCategory string             `json:"eventTypeCategory"`
	EventTypeCode     string             `json:"eventTypeCode"`
	Service           string             `json:"service"`
}

// HealthEventURL returns the unique unescaped URL asscociated with an AWS health event
func (h *Event) HealthEventURL() string {
	return fmt.Sprintf("%s#/dashboard/open-issues?eventID=%s&eventTab=details&layout=horizontal", PersonalHealthDashboardURL, h.EventARN)
}
