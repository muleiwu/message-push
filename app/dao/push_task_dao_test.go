package dao

import (
	"path/filepath"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestPushTaskListFiltersReceiverAndPreloadsHistoricalRelations(t *testing.T) {
	db := newPushTaskListTestDB(t)
	for _, statement := range []string{
		`INSERT INTO channels (id, name, type, status, deleted_at) VALUES (1, '历史短信通道', 'sms', 0, CURRENT_TIMESTAMP)`,
		`INSERT INTO provider_accounts (id, account_code, account_name, provider_code, provider_type, config, status, deleted_at) VALUES (1, 'history', '历史服务商账号', 'aliyun_sms', 'sms', '{}', 0, CURRENT_TIMESTAMP)`,
		`INSERT INTO push_tasks (task_id, app_id, channel_id, provider_account_id, message_type, receiver, status) VALUES ('literal-match', 'app', 1, 1, 'email', 'user_100%@example.com', 'success')`,
		`INSERT INTO push_tasks (task_id, app_id, channel_id, message_type, receiver, status) VALUES ('wildcard-lookalike', 'app', 1, 'email', 'userX100ZZ@example.com', 'pending')`,
		`INSERT INTO push_tasks (task_id, app_id, channel_id, message_type, receiver, status) VALUES ('phone-one', 'app', 1, 'sms', '+8613800138000', 'success')`,
		`INSERT INTO push_tasks (task_id, app_id, channel_id, message_type, receiver, status) VALUES ('phone-two', 'app', 1, 'sms', '+8613800138001', 'success')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed test data: %v", err)
		}
	}

	t.Run("escapes wildcard characters", func(t *testing.T) {
		items, total, err := NewPushTaskDAOWithDB(db).List(1, 20, map[string]interface{}{
			"receiver": "_100%",
		})
		if err != nil {
			t.Fatalf("list tasks: %v", err)
		}
		if total != 1 || len(items) != 1 || items[0].TaskID != "literal-match" {
			t.Fatalf("literal receiver filter returned total=%d items=%+v", total, items)
		}
		if items[0].Channel == nil || items[0].Channel.Name != "历史短信通道" {
			t.Fatalf("historical channel not preloaded: %+v", items[0].Channel)
		}
		if items[0].ProviderAccount == nil || items[0].ProviderAccount.AccountName != "历史服务商账号" {
			t.Fatalf("historical provider not preloaded: %+v", items[0].ProviderAccount)
		}
	})

	t.Run("returns contains matches with total before pagination", func(t *testing.T) {
		items, total, err := NewPushTaskDAOWithDB(db).List(1, 1, map[string]interface{}{
			"receiver": "1380013800",
		})
		if err != nil {
			t.Fatalf("list tasks: %v", err)
		}
		if total != 2 || len(items) != 1 {
			t.Fatalf("receiver pagination total=%d len=%d, want 2/1", total, len(items))
		}
	})
}

func TestPushTaskListDateFilterUsesShanghaiBusinessDay(t *testing.T) {
	db := newPushTaskListTestDB(t)
	for _, statement := range []string{
		`INSERT INTO push_tasks (task_id, app_id, channel_id, message_type, receiver, status, created_at) VALUES ('before', 'app', 1, 'sms', '1', 'success', '2026-08-03 15:59:59+00:00')`,
		`INSERT INTO push_tasks (task_id, app_id, channel_id, message_type, receiver, status, created_at) VALUES ('start', 'app', 1, 'sms', '2', 'success', '2026-08-03 16:00:00+00:00')`,
		`INSERT INTO push_tasks (task_id, app_id, channel_id, message_type, receiver, status, created_at) VALUES ('end', 'app', 1, 'sms', '3', 'success', '2026-08-04 15:59:59+00:00')`,
		`INSERT INTO push_tasks (task_id, app_id, channel_id, message_type, receiver, status, created_at) VALUES ('after', 'app', 1, 'sms', '4', 'success', '2026-08-04 16:00:00+00:00')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("seed dated task: %v", err)
		}
	}

	items, total, err := NewPushTaskDAOWithDB(db).List(1, 20, map[string]interface{}{
		"start_date": "2026-08-04",
		"end_date":   "2026-08-04",
	})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("business-day filter returned total=%d items=%+v", total, items)
	}
	for _, item := range items {
		if item.TaskID != "start" && item.TaskID != "end" {
			t.Fatalf("business-day filter included %q", item.TaskID)
		}
	}
}

func TestGetScheduledTasksComparesAbsoluteInstants(t *testing.T) {
	db := newPushTaskListTestDB(t)
	shanghaiOffset := time.FixedZone("UTC+8", 8*60*60)
	now := time.Now().UTC()
	dao := NewPushTaskDAOWithDB(db)
	for _, task := range []struct {
		id          string
		scheduledAt time.Time
	}{
		{id: "due", scheduledAt: now.Add(-time.Minute).In(shanghaiOffset)},
		{id: "future", scheduledAt: now.Add(time.Minute).In(shanghaiOffset)},
	} {
		if err := dao.Create(&model.PushTask{
			TaskID:      task.id,
			AppID:       "app",
			ChannelID:   1,
			MessageType: "sms",
			Receiver:    "1",
			Status:      "pending",
			ScheduledAt: &task.scheduledAt,
		}); err != nil {
			t.Fatalf("seed scheduled task %q: %v", task.id, err)
		}
	}

	tasks, err := dao.GetScheduledTasks(10)
	if err != nil {
		t.Fatalf("get scheduled tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].TaskID != "due" {
		t.Fatalf("scheduled tasks = %+v, want only due task", tasks)
	}
}

func newPushTaskListTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "push-task-list.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	for _, statement := range []string{
		`CREATE TABLE channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, type TEXT NOT NULL,
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
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}
	return db
}
