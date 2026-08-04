package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAdminTaskServiceReturnsChannelAndLastProvider(t *testing.T) {
	db := newAdminTaskTestDB(t)
	for _, statement := range []string{
		`INSERT INTO channels (id, name, type, status) VALUES (1, '验证码通道', 'sms', 1)`,
		`INSERT INTO provider_accounts (id, account_code, account_name, provider_code, provider_type, config, status) VALUES (1, 'aliyun-main', '阿里云主账号', 'aliyun_sms', 'sms', '{}', 1)`,
		`INSERT INTO push_tasks (task_id, app_id, channel_id, provider_account_id, message_type, receiver, status) VALUES ('task-with-provider', 'app', 1, 1, 'sms', '+8613800138000', 'success')`,
		`INSERT INTO push_tasks (task_id, app_id, channel_id, message_type, receiver, status) VALUES ('task-without-provider', 'app', 1, 'sms', '+8613900139000', 'pending')`,
		`INSERT INTO push_logs (task_id, app_id, provider_account_id, provider_msg_id, status) VALUES ('task-with-provider', 'app', 1, 'old-message', 'failed')`,
		`INSERT INTO push_logs (task_id, app_id, provider_account_id, provider_msg_id, status) VALUES ('task-with-provider', 'app', 1, 'latest-message', 'success')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed test data: %v", err)
		}
	}

	service := &AdminTaskService{
		pushTaskDAO: dao.NewPushTaskDAOWithDB(db),
		pushLogDAO:  dao.NewPushLogDAOWithDB(db),
	}
	response, err := service.GetPushTaskList(&dto.PushTaskListRequest{
		Page:     1,
		PageSize: 20,
		Receiver: "13800138",
	})
	if err != nil {
		t.Fatalf("get task list: %v", err)
	}
	if response.Total != 1 || len(response.Items) != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}
	item := response.Items[0]
	if item.ChannelName != "验证码通道" || item.ProviderAccountName != "阿里云主账号" {
		t.Fatalf("unexpected related names: %+v", item)
	}
	if item.ProviderAccountID == nil || *item.ProviderAccountID != 1 {
		t.Fatalf("provider account id = %v, want 1", item.ProviderAccountID)
	}
	if item.ProviderMsgID != "latest-message" {
		t.Fatalf("provider message id = %q, want latest-message", item.ProviderMsgID)
	}

	pending, err := service.GetPushTaskList(&dto.PushTaskListRequest{
		Page:     1,
		PageSize: 20,
		Receiver: "13900139",
	})
	if err != nil {
		t.Fatalf("get pending task: %v", err)
	}
	if len(pending.Items) != 1 || pending.Items[0].ProviderAccountID != nil || pending.Items[0].ProviderAccountName != "" {
		t.Fatalf("pending task unexpectedly has a provider: %+v", pending.Items)
	}
}

func TestAdminTaskServiceReturnsUTCWireTimes(t *testing.T) {
	db := newAdminTaskTestDB(t)
	if err := db.Exec(`
		INSERT INTO push_tasks (
			task_id, app_id, channel_id, message_type, receiver, status,
			callback_time, scheduled_at, created_at, updated_at
		) VALUES (
			'utc-wire-task', 'app', 1, 'sms', '+8613800138000', 'success',
			'2026-08-04 18:00:00+08:00', '2026-08-04 19:00:00+08:00',
			'2026-08-04 18:00:00+08:00', '2026-08-04 18:05:00+08:00'
		)
	`).Error; err != nil {
		t.Fatalf("seed UTC wire task: %v", err)
	}

	service := &AdminTaskService{
		pushTaskDAO: dao.NewPushTaskDAOWithDB(db),
		pushLogDAO:  dao.NewPushLogDAOWithDB(db),
	}
	response, err := service.GetPushTaskList(&dto.PushTaskListRequest{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("get task list: %v", err)
	}
	if len(response.Items) != 1 {
		t.Fatalf("task count = %d, want 1", len(response.Items))
	}
	item := response.Items[0]
	if item.CreatedAt != "2026-08-04T10:00:00Z" || item.UpdatedAt != "2026-08-04T10:05:00Z" {
		t.Fatalf("wire timestamps = created %q updated %q", item.CreatedAt, item.UpdatedAt)
	}
	if item.CallbackTime == nil || item.CallbackTime.Location() != time.UTC ||
		item.ScheduledAt == nil || item.ScheduledAt.Location() != time.UTC {
		t.Fatalf("pointer timestamps are not UTC: callback=%v scheduled=%v", item.CallbackTime, item.ScheduledAt)
	}

	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if strings.Contains(string(payload), "+08:00") {
		t.Fatalf("response contains non-UTC offset: %s", payload)
	}
}

func newAdminTaskTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "admin-task.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, type TEXT,
			message_template_id INTEGER, status INTEGER, created_at DATETIME,
			updated_at DATETIME, deleted_at DATETIME
		)`,
		`CREATE TABLE provider_accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT, account_code TEXT, account_name TEXT,
			provider_code TEXT, provider_type TEXT, config TEXT, status INTEGER,
			remark TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME
		)`,
		`CREATE TABLE push_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL UNIQUE,
			app_id TEXT NOT NULL, channel_id INTEGER NOT NULL, provider_account_id INTEGER,
			message_type TEXT NOT NULL, receiver TEXT NOT NULL, template_code TEXT,
			template_params TEXT, signature TEXT, status TEXT, callback_status TEXT,
			callback_time DATETIME, retry_count INTEGER, max_retry INTEGER,
			exclude_provider_ids TEXT, scheduled_at DATETIME, created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE push_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, app_id TEXT NOT NULL,
			provider_account_id INTEGER NOT NULL, provider_msg_id TEXT, request_data TEXT,
			response_data TEXT, status TEXT, error_message TEXT, cost_time INTEGER,
			created_at DATETIME
		)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}
