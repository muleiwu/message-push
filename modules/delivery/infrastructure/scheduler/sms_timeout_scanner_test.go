package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/service"
	"github.com/glebarez/sqlite"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
)

func TestScanProcessingTasksCreatesOneFailedOutbox(t *testing.T) {
	db := newTimeoutScannerTestDB(t)
	if err := db.Create(&model.Application{
		AppID:      "timeout-app",
		AppName:    "timeout",
		AppSecret:  "secret",
		WebhookURL: "https://application.example/webhook",
	}).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	task := &model.PushTask{
		TaskID:      "processing-timeout",
		AppID:       "timeout-app",
		ChannelID:   1,
		MessageType: constants.MessageTypeEmail,
		Receiver:    "user@example.com",
		Status:      constants.TaskStatusProcessing,
		UpdatedAt:   time.Now().In(time.FixedZone("UTC+8", 8*60*60)).Add(-10 * time.Minute),
	}
	taskDAO := dao.NewPushTaskDAOWithDB(db)
	if err := taskDAO.Create(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	scanner := &SMSTimeoutScanner{
		logger:            timeoutScannerNoopLogger{},
		taskDao:           taskDAO,
		processingTimeout: time.Minute,
		limit:             100,
		terminalService:   service.NewTaskTerminalServiceWithDB(db),
	}
	scanner.scanProcessingTasks(context.Background())
	scanner.scanProcessingTasks(context.Background())

	var saved model.PushTask
	if err := db.Where("task_id = ?", task.TaskID).First(&saved).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if saved.Status != constants.TaskStatusFailed {
		t.Fatalf("task status = %q, want failed", saved.Status)
	}
	var outboxes []model.WebhookLog
	if err := db.Where("task_id = ?", task.TaskID).Find(&outboxes).Error; err != nil {
		t.Fatalf("load outboxes: %v", err)
	}
	if len(outboxes) != 1 || outboxes[0].Event != constants.WebhookEventFailed {
		t.Fatalf("outboxes = %+v, want one failed event", outboxes)
	}
}

func newTimeoutScannerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "timeout.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	statements := []string{
		`CREATE TABLE applications (
			id INTEGER PRIMARY KEY AUTOINCREMENT, app_id TEXT NOT NULL UNIQUE, app_secret TEXT NOT NULL,
			app_name TEXT NOT NULL, status INTEGER, ip_whitelist TEXT, webhook_url TEXT,
			daily_quota INTEGER, rate_limit INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		)`,
		`CREATE TABLE push_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL UNIQUE, app_id TEXT NOT NULL,
			channel_id INTEGER NOT NULL, provider_account_id INTEGER, message_type TEXT NOT NULL, receiver TEXT NOT NULL,
			template_code TEXT, template_params TEXT, signature TEXT, status TEXT,
			callback_status TEXT, callback_time DATETIME, retry_count INTEGER, max_retry INTEGER,
			exclude_provider_ids TEXT, scheduled_at DATETIME, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE webhook_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, app_id TEXT NOT NULL UNIQUE, webhook_url TEXT NOT NULL,
			secret TEXT, events TEXT, status INTEGER, retry_count INTEGER, timeout INTEGER,
			description TEXT, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE webhook_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT, app_id TEXT NOT NULL,
			webhook_config_id INTEGER, webhook_url TEXT NOT NULL, event TEXT NOT NULL,
			request_data TEXT, response_status INTEGER, response_data TEXT, status TEXT NOT NULL,
			error_message TEXT, retry_count INTEGER, dedup_key TEXT UNIQUE, signing_secret TEXT,
			max_retries INTEGER, timeout_seconds INTEGER, next_attempt_at DATETIME,
			locked_until DATETIME, lease_token TEXT, created_at DATETIME, updated_at DATETIME
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	return db
}

type timeoutScannerNoopLogger struct{}

func (timeoutScannerNoopLogger) Debug(string, ...gsr.LoggerField)  {}
func (timeoutScannerNoopLogger) Info(string, ...gsr.LoggerField)   {}
func (timeoutScannerNoopLogger) Notice(string, ...gsr.LoggerField) {}
func (timeoutScannerNoopLogger) Error(string, ...gsr.LoggerField)  {}
func (timeoutScannerNoopLogger) Warn(string, ...gsr.LoggerField)   {}
func (timeoutScannerNoopLogger) Fatal(string, ...gsr.LoggerField)  {}
