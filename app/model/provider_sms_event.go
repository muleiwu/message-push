package model

import "time"

// ProviderSMSEvent is the durable inbox for provider facts. Effect contains only
// deferred delivery work; ingestion never executes rules or sends webhooks.
type ProviderSMSEvent struct {
	ID                uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	EventKey          string     `gorm:"size:64;not null;uniqueIndex:uk_provider_sms_events_key" json:"event_key"`
	ProviderAccountID uint       `gorm:"not null;index:idx_provider_sms_events_account" json:"provider_account_id"`
	SDKAppID          string     `gorm:"column:sdk_app_id;size:100;not null" json:"sdk_app_id"`
	Kind              string     `gorm:"size:20;not null" json:"kind"`
	Source            string     `gorm:"size:20;not null" json:"source"`
	ProviderMsgID     string     `gorm:"size:100" json:"provider_msg_id"`
	Mobile            string     `gorm:"size:30" json:"mobile"`
	Status            string     `gorm:"size:30" json:"status"`
	ErrorCode         string     `gorm:"size:200" json:"error_code"`
	ErrorMessage      string     `gorm:"type:text" json:"error_message"`
	OccurredAt        *time.Time `json:"occurred_at"`
	Content           string     `gorm:"type:text" json:"content"`
	SignName          string     `gorm:"size:200" json:"sign_name"`
	ExtendCode        string     `gorm:"size:100" json:"extend_code"`
	RawData           string     `gorm:"type:text" json:"raw_data"`
	RequestID         string     `gorm:"size:100" json:"request_id"`
	State             string     `gorm:"size:20;not null;index:idx_provider_sms_events_dispatch" json:"state"`
	Attempts          int        `gorm:"not null;default:0" json:"attempts"`
	NextAttemptAt     *time.Time `gorm:"index:idx_provider_sms_events_dispatch" json:"next_attempt_at"`
	LastError         string     `gorm:"type:text" json:"last_error"`
	AppID             string     `gorm:"size:32" json:"app_id"`
	Attribution       string     `gorm:"size:30" json:"attribution"`
	Effect            string     `gorm:"type:text" json:"-"`
	ProcessedAt       *time.Time `json:"processed_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (ProviderSMSEvent) TableName() string { return "provider_sms_events" }

// SMSPullRun is created before calling Tencent. A stale started run represents
// an uncertain consumption; it must not silently trigger another cloud call.
type SMSPullRun struct {
	ID                uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ProviderAccountID uint       `gorm:"not null;index:idx_provider_sms_pull_runs_account" json:"provider_account_id"`
	SDKAppID          string     `gorm:"column:sdk_app_id;size:100;not null" json:"sdk_app_id"`
	Kind              string     `gorm:"size:20;not null" json:"kind"`
	Source            string     `gorm:"size:20;not null" json:"source"`
	State             string     `gorm:"size:20;not null;index:idx_provider_sms_pull_runs_state" json:"state"`
	RequestID         string     `gorm:"size:100" json:"request_id"`
	Received          int        `json:"received"`
	Inserted          int        `json:"inserted"`
	Duplicates        int        `json:"duplicates"`
	ErrorCode         string     `gorm:"size:200" json:"error_code"`
	ErrorMessage      string     `gorm:"type:text" json:"error_message"`
	StartedAt         time.Time  `gorm:"index:idx_provider_sms_pull_runs_state" json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
}

func (SMSPullRun) TableName() string { return "provider_sms_pull_runs" }

type SMSPollingStream struct {
	ID                uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ProviderAccountID uint       `gorm:"not null;uniqueIndex:uk_provider_sms_polling_account_kind" json:"provider_account_id"`
	Kind              string     `gorm:"size:20;not null;uniqueIndex:uk_provider_sms_polling_account_kind" json:"kind"`
	SDKAppID          string     `gorm:"column:sdk_app_id;size:100;not null" json:"sdk_app_id"`
	OwnerKey          *string    `gorm:"size:150;uniqueIndex:uk_provider_sms_polling_owner" json:"-"`
	Enabled           bool       `gorm:"not null;default:false" json:"enabled"`
	IntervalSeconds   int        `gorm:"not null;default:30" json:"interval_seconds"`
	Suspended         bool       `gorm:"not null;default:false" json:"suspended"`
	SuspendReason     string     `gorm:"type:text" json:"suspend_reason"`
	NextPollAt        *time.Time `gorm:"index:idx_provider_sms_polling_due" json:"next_poll_at"`
	Failures          int        `gorm:"not null;default:0" json:"failures"`
	LastSuccessAt     *time.Time `json:"last_success_at"`
	LastRunID         uint64     `json:"last_run_id"`
	LastError         string     `gorm:"type:text" json:"last_error"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (SMSPollingStream) TableName() string { return "provider_sms_polling" }
