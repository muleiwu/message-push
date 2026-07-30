package worker

import (
	"context"
	"path/filepath"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/modules/channel"
	"cnb.cool/mliev/push/message-push/modules/sender"
	"github.com/glebarez/sqlite"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
)

func TestHandleEarlyFailureCreatesTerminalWebhookOutbox(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "worker-webhook.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE applications (id INTEGER PRIMARY KEY AUTOINCREMENT, app_id TEXT NOT NULL UNIQUE, webhook_url TEXT)`,
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
		`CREATE TABLE push_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, app_id TEXT NOT NULL,
			provider_account_id INTEGER NOT NULL, provider_msg_id TEXT, request_data TEXT,
			response_data TEXT, status TEXT NOT NULL, error_message TEXT, cost_time INTEGER,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}

	task := &model.PushTask{
		TaskID:      "worker-final-failure",
		AppID:       "app-worker-failure",
		ChannelID:   1,
		MessageType: constants.MessageTypeSMS,
		Receiver:    "13800138000",
		Status:      constants.TaskStatusProcessing,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := db.Create(&model.WebhookConfig{
		AppID:      task.AppID,
		WebhookURL: "https://example.com/webhook",
		Events:     constants.TaskStatusFailed,
		Status:     1,
		RetryCount: 3,
		Timeout:    5,
	}).Error; err != nil {
		t.Fatalf("create webhook config: %v", err)
	}

	handler := &MessageHandler{
		logger:          noopLogger{},
		taskDao:         dao.NewPushTaskDAOWithDB(db),
		terminalService: service.NewTaskTerminalServiceWithDB(db),
	}
	handler.handleEarlyFailure(task, 0, "provider unavailable")

	var outboxCount int64
	if err := db.Model(&model.WebhookLog{}).
		Where("task_id = ? AND event = ?", task.TaskID, constants.TaskStatusFailed).
		Count(&outboxCount).Error; err != nil {
		t.Fatalf("count webhook outbox: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("terminal webhook outbox rows = %d, want 1", outboxCount)
	}
}

func TestHandleSuccessCreatesOutboxOnlyForTerminalStatus(t *testing.T) {
	db := newWorkerWebhookTestDB(t)
	terminalService := service.NewTaskTerminalServiceWithDB(db)
	handler := &MessageHandler{
		logger:          noopLogger{},
		taskDao:         dao.NewPushTaskDAOWithDB(db),
		logDao:          dao.NewPushLogDAOWithDB(db),
		selector:        noopSelector{},
		terminalService: terminalService,
	}
	if err := db.Create(&model.WebhookConfig{
		AppID:      "app-worker-success",
		WebhookURL: "https://example.com/webhook",
		Events:     constants.WebhookEventSuccess,
		Status:     1,
		RetryCount: 3,
		Timeout:    5,
	}).Error; err != nil {
		t.Fatalf("create webhook config: %v", err)
	}

	terminalTask := createWorkerWebhookTask(t, db, "worker-success", "app-worker-success")
	if err := handler.handleSuccess(terminalTask, 1, &sender.SendResponse{
		Success:    true,
		ProviderID: "provider-success",
		Status:     constants.TaskStatusSuccess,
	}); err != nil {
		t.Fatalf("handle terminal success: %v", err)
	}
	nonTerminalTask := createWorkerWebhookTask(t, db, "worker-sent", "app-worker-success")
	if err := handler.handleSuccess(nonTerminalTask, 1, &sender.SendResponse{
		Success:    true,
		ProviderID: "provider-sent",
		Status:     constants.TaskStatusSent,
	}); err != nil {
		t.Fatalf("handle non-terminal success: %v", err)
	}

	var outboxes []model.WebhookLog
	if err := db.Order("id ASC").Find(&outboxes).Error; err != nil {
		t.Fatalf("load outboxes: %v", err)
	}
	if len(outboxes) != 1 || outboxes[0].TaskID != terminalTask.TaskID ||
		outboxes[0].Event != constants.WebhookEventSuccess {
		t.Fatalf("unexpected outboxes: %+v", outboxes)
	}
	var savedSent model.PushTask
	if err := db.Where("task_id = ?", nonTerminalTask.TaskID).First(&savedSent).Error; err != nil {
		t.Fatalf("load sent task: %v", err)
	}
	if savedSent.Status != constants.TaskStatusSent {
		t.Fatalf("non-terminal task status = %q, want sent", savedSent.Status)
	}
}

func newWorkerWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "worker-webhook.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range workerWebhookTestSchema {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	return db
}

func createWorkerWebhookTask(t *testing.T, db *gorm.DB, taskID, appID string) *model.PushTask {
	t.Helper()
	task := &model.PushTask{
		TaskID:      taskID,
		AppID:       appID,
		ChannelID:   1,
		MessageType: constants.MessageTypeSMS,
		Receiver:    "13800138000",
		Status:      constants.TaskStatusProcessing,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	return task
}

var workerWebhookTestSchema = []string{
	`CREATE TABLE applications (id INTEGER PRIMARY KEY AUTOINCREMENT, app_id TEXT NOT NULL UNIQUE, webhook_url TEXT)`,
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
	`CREATE TABLE push_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, app_id TEXT NOT NULL,
		provider_account_id INTEGER NOT NULL, provider_msg_id TEXT, request_data TEXT,
		response_data TEXT, status TEXT NOT NULL, error_message TEXT, cost_time INTEGER,
		created_at DATETIME
	)`,
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...gsr.LoggerField)  {}
func (noopLogger) Info(string, ...gsr.LoggerField)   {}
func (noopLogger) Notice(string, ...gsr.LoggerField) {}
func (noopLogger) Error(string, ...gsr.LoggerField)  {}
func (noopLogger) Warn(string, ...gsr.LoggerField)   {}
func (noopLogger) Fatal(string, ...gsr.LoggerField)  {}

type noopSelector struct{}

func (noopSelector) Select(context.Context, uint, string, string, string) (*channel.ChannelNode, error) {
	return nil, nil
}
func (noopSelector) SelectWithExcludes(context.Context, uint, string, string, string, []uint) (*channel.ChannelNode, error) {
	return nil, nil
}
func (noopSelector) ReportSuccess(uint)             {}
func (noopSelector) ReportFailure(uint)             {}
func (noopSelector) ResetWeightsByChannelID(uint)   {}
func (noopSelector) ClearCache()                    {}
func (noopSelector) ClearCacheByChannelID(uint)     {}
func (noopSelector) ClearCacheByKey(uint, string)   {}
func (noopSelector) InvalidateCacheForBinding(uint) {}
