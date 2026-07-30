package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestTaskTerminalServiceTransitionCreatesOneOutbox(t *testing.T) {
	db := newTerminalServiceTestDB(t, true)
	task := createTerminalTestTask(t, db, "terminal-once", "app-terminal-once")
	if err := db.Create(&model.WebhookConfig{
		AppID:      task.AppID,
		WebhookURL: "https://explicit.example/webhook",
		Secret:     "secret",
		Events:     "failed",
		Status:     1,
		RetryCount: 2,
		Timeout:    7,
	}).Error; err != nil {
		t.Fatalf("create webhook config: %v", err)
	}

	service := NewTaskTerminalServiceWithDB(db)
	occurredAt := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	persistedAt := occurredAt.Add(5 * time.Minute)
	service.now = func() time.Time { return persistedAt }
	first, err := service.Transition(context.Background(), TerminalTransition{
		TaskID:       task.TaskID,
		Status:       constants.TaskStatusFailed,
		Event:        constants.WebhookEventFailed,
		ErrorCode:    "SEND_FAILED",
		ErrorMessage: "provider rejected request",
		ProviderID:   "provider-message-1",
		OccurredAt:   occurredAt,
	})
	if err != nil {
		t.Fatalf("first transition: %v", err)
	}
	if !first.Changed || first.OutboxID == 0 {
		t.Fatalf("first transition = %+v, want changed with outbox", first)
	}

	second, err := service.Transition(context.Background(), TerminalTransition{
		TaskID:     task.TaskID,
		Status:     constants.TaskStatusSuccess,
		Event:      constants.WebhookEventSuccess,
		OccurredAt: occurredAt.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("duplicate transition: %v", err)
	}
	if second.Changed {
		t.Fatalf("duplicate terminal transition changed task: %+v", second)
	}

	var savedTask model.PushTask
	if err := db.Where("task_id = ?", task.TaskID).First(&savedTask).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if savedTask.Status != constants.TaskStatusFailed {
		t.Fatalf("task status = %q, want failed", savedTask.Status)
	}

	var outboxes []model.WebhookLog
	if err := db.Where("task_id = ?", task.TaskID).Find(&outboxes).Error; err != nil {
		t.Fatalf("load outboxes: %v", err)
	}
	if len(outboxes) != 1 {
		t.Fatalf("outbox count = %d, want 1", len(outboxes))
	}
	outbox := outboxes[0]
	if outbox.Status != constants.WebhookDeliveryPending ||
		outbox.DedupKey != "task:"+task.TaskID+":terminal" ||
		outbox.SigningSecret != "secret" ||
		outbox.MaxRetries != 2 ||
		outbox.TimeoutSeconds != 7 {
		t.Fatalf("unexpected outbox snapshot: %+v", outbox)
	}
	if outbox.NextAttemptAt == nil || !outbox.NextAttemptAt.Equal(persistedAt) ||
		!outbox.CreatedAt.Equal(persistedAt) {
		t.Fatalf("outbox enqueue time = next %v created %v, want %v", outbox.NextAttemptAt, outbox.CreatedAt, persistedAt)
	}
	var payload struct {
		Timestamp int64 `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(outbox.RequestData), &payload); err != nil {
		t.Fatalf("decode webhook payload: %v", err)
	}
	if payload.Timestamp != occurredAt.Unix() {
		t.Fatalf("payload timestamp = %d, want event time %d", payload.Timestamp, occurredAt.Unix())
	}
	if !savedTask.UpdatedAt.Equal(persistedAt) {
		t.Fatalf("task updated_at = %v, want persistence time %v", savedTask.UpdatedAt, persistedAt)
	}
}

func TestTaskTerminalServiceApplicationFallbackAndExplicitDisable(t *testing.T) {
	tests := []struct {
		name       string
		config     *model.WebhookConfig
		wantOutbox bool
	}{
		{
			name:       "application URL is used when advanced config is absent",
			wantOutbox: true,
		},
		{
			name: "disabled advanced config suppresses application fallback",
			config: &model.WebhookConfig{
				WebhookURL: "https://disabled.example/webhook",
				Events:     constants.DefaultWebhookEvents,
				Status:     0,
			},
			wantOutbox: false,
		},
		{
			name: "unsubscribed advanced config suppresses application fallback",
			config: &model.WebhookConfig{
				WebhookURL: "https://explicit.example/webhook",
				Events:     constants.WebhookEventSuccess,
				Status:     1,
			},
			wantOutbox: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTerminalServiceTestDB(t, true)
			task := createTerminalTestTask(t, db, "fallback-task", "fallback-app")
			if err := db.Create(&model.Application{
				AppID:      task.AppID,
				AppName:    "fallback",
				AppSecret:  "secret",
				WebhookURL: "https://application.example/webhook",
			}).Error; err != nil {
				t.Fatalf("create application: %v", err)
			}
			if tt.config != nil {
				disabled := tt.config.Status == 0
				tt.config.AppID = task.AppID
				if err := db.Create(tt.config).Error; err != nil {
					t.Fatalf("create webhook config: %v", err)
				}
				if disabled {
					if err := db.Model(tt.config).UpdateColumn("status", 0).Error; err != nil {
						t.Fatalf("disable webhook config: %v", err)
					}
				}
			}

			result, err := NewTaskTerminalServiceWithDB(db).Transition(context.Background(), TerminalTransition{
				TaskID:       task.TaskID,
				Status:       constants.TaskStatusFailed,
				Event:        constants.WebhookEventFailed,
				ErrorMessage: "failed",
			})
			if err != nil {
				t.Fatalf("transition: %v", err)
			}
			if !result.Changed {
				t.Fatal("task did not transition")
			}
			if (result.OutboxID != 0) != tt.wantOutbox {
				t.Fatalf("outbox id = %d, wantOutbox=%v", result.OutboxID, tt.wantOutbox)
			}
		})
	}
}

func TestTaskTerminalServiceRollsBackWhenOutboxInsertFails(t *testing.T) {
	db := newTerminalServiceTestDB(t, false)
	task := createTerminalTestTask(t, db, "rollback-task", "rollback-app")
	if err := db.Create(&model.Application{
		AppID:      task.AppID,
		AppName:    "rollback",
		AppSecret:  "secret",
		WebhookURL: "https://application.example/webhook",
	}).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}

	_, err := NewTaskTerminalServiceWithDB(db).Transition(context.Background(), TerminalTransition{
		TaskID:       task.TaskID,
		Status:       constants.TaskStatusFailed,
		Event:        constants.WebhookEventFailed,
		ErrorMessage: "must roll back",
	})
	if err == nil {
		t.Fatal("transition unexpectedly succeeded without webhook_logs table")
	}

	var saved model.PushTask
	if err := db.Where("task_id = ?", task.TaskID).First(&saved).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if saved.Status != constants.TaskStatusProcessing {
		t.Fatalf("task status = %q, want processing after rollback", saved.Status)
	}
}

func TestTaskTerminalServiceRecordUpstreamUsesApplicationFallback(t *testing.T) {
	db := newTerminalServiceTestDB(t, true)
	if err := db.Create(&model.Application{
		AppID:      "upstream-app",
		AppName:    "upstream",
		AppSecret:  "secret",
		WebhookURL: "https://application.example/webhook",
	}).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}
	receiveTime := time.Date(2026, 7, 30, 12, 30, 0, 0, time.UTC)
	if err := NewTaskTerminalServiceWithDB(db).RecordUpstream(context.Background(), UpstreamEvent{
		AppID:        "upstream-app",
		Mobile:       "13800138000",
		Content:      "STOP",
		ProviderCode: "provider",
		ReceiveTime:  receiveTime,
		RawData:      `{"content":"STOP"}`,
	}); err != nil {
		t.Fatalf("record upstream: %v", err)
	}

	var callback model.CallbackLog
	if err := db.First(&callback).Error; err != nil {
		t.Fatalf("load callback: %v", err)
	}
	var outbox model.WebhookLog
	if err := db.First(&outbox).Error; err != nil {
		t.Fatalf("load outbox: %v", err)
	}
	if outbox.Event != constants.WebhookEventUpstream ||
		outbox.DedupKey != "callback:"+itoaForTest(callback.ID)+":upstream" {
		t.Fatalf("unexpected upstream outbox: %+v", outbox)
	}
}

func TestTaskTerminalServiceRecordUpstreamRollsBackCallbackWhenOutboxFails(t *testing.T) {
	db := newTerminalServiceTestDB(t, false)
	if err := db.Create(&model.Application{
		AppID:      "rollback-upstream-app",
		AppName:    "rollback-upstream",
		AppSecret:  "secret",
		WebhookURL: "https://application.example/webhook",
	}).Error; err != nil {
		t.Fatalf("create application: %v", err)
	}

	err := NewTaskTerminalServiceWithDB(db).RecordUpstream(context.Background(), UpstreamEvent{
		AppID:        "rollback-upstream-app",
		Mobile:       "13800138000",
		Content:      "STOP",
		ProviderCode: "provider",
	})
	if err == nil {
		t.Fatal("record upstream unexpectedly succeeded without webhook_logs table")
	}

	var callbackCount int64
	if err := db.Model(&model.CallbackLog{}).Count(&callbackCount).Error; err != nil {
		t.Fatalf("count callbacks: %v", err)
	}
	if callbackCount != 0 {
		t.Fatalf("callback count = %d, want 0 after rollback", callbackCount)
	}
}

func newTerminalServiceTestDB(t *testing.T, withWebhookLogs bool) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "terminal.db")), &gorm.Config{})
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
			channel_id INTEGER NOT NULL, message_type TEXT NOT NULL, receiver TEXT NOT NULL,
			template_code TEXT, template_params TEXT, signature TEXT, status TEXT,
			callback_status TEXT, callback_time DATETIME, retry_count INTEGER, max_retry INTEGER,
			exclude_provider_ids TEXT, scheduled_at DATETIME, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE webhook_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, app_id TEXT NOT NULL UNIQUE, webhook_url TEXT NOT NULL,
			secret TEXT, events TEXT, status INTEGER, retry_count INTEGER, timeout INTEGER,
			description TEXT, created_at DATETIME, updated_at DATETIME
		)`,
		`CREATE TABLE callback_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, type TEXT, task_id TEXT, app_id TEXT NOT NULL,
			provider_code TEXT NOT NULL, provider_id TEXT, mobile TEXT, content TEXT,
			callback_status TEXT, error_code TEXT, error_message TEXT, raw_data TEXT, created_at DATETIME
		)`,
		`CREATE TABLE push_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, app_id TEXT NOT NULL,
			provider_account_id INTEGER NOT NULL, provider_msg_id TEXT, request_data TEXT,
			response_data TEXT, status TEXT NOT NULL, error_message TEXT, cost_time INTEGER,
			created_at DATETIME
		)`,
	}
	if withWebhookLogs {
		statements = append(statements, `CREATE TABLE webhook_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT, app_id TEXT NOT NULL,
			webhook_config_id INTEGER, webhook_url TEXT NOT NULL, event TEXT NOT NULL,
			request_data TEXT, response_status INTEGER, response_data TEXT, status TEXT NOT NULL,
			error_message TEXT, retry_count INTEGER, dedup_key TEXT UNIQUE, signing_secret TEXT,
			max_retries INTEGER, timeout_seconds INTEGER, next_attempt_at DATETIME,
			locked_until DATETIME, lease_token TEXT, created_at DATETIME, updated_at DATETIME
		)`)
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	return db
}

func createTerminalTestTask(t *testing.T, db *gorm.DB, taskID, appID string) *model.PushTask {
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

func itoaForTest(value uint) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	index := len(buf)
	for value > 0 {
		index--
		buf[index] = digits[value%10]
		value /= 10
	}
	return string(buf[index:])
}
