package domain

import (
	"context"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
)

const (
	SMSReports = "reports"
	SMSReplies = "replies"
)

// SMSEvent is a provider fact, before application attribution or task mutation.
type SMSEvent struct {
	ProviderMsgID string    `json:"provider_msg_id,omitempty"`
	Mobile        string    `json:"mobile"`
	Status        string    `json:"status,omitempty"`
	ErrorCode     string    `json:"error_code,omitempty"`
	ErrorMessage  string    `json:"error_message,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
	Content       string    `json:"content,omitempty"`
	SignName      string    `json:"sign_name,omitempty"`
	ExtendCode    string    `json:"extend_code,omitempty"`
	RawData       string    `json:"-"`
}

type SMSEventRequest struct {
	Account     *model.ProviderAccount
	Kind        string
	PhoneNumber string
	BeginTime   time.Time
	EndTime     time.Time
	Limit       int
}

type SMSEventResponse struct {
	Items        []SMSEvent `json:"items"`
	RequestID    string     `json:"request_id"`
	LimitReached bool       `json:"limit_reached"`
}

// SMSReporter exposes historical queries and destructive queue consumption
// separately. Implementations must never retry an uncertain PullSMSEvents.
type SMSReporter interface {
	QuerySMSEvents(context.Context, *SMSEventRequest) (*SMSEventResponse, error)
	PullSMSEvents(context.Context, *SMSEventRequest) (*SMSEventResponse, error)
}
