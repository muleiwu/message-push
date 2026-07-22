package dao

import (
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestPushLogListFiltersProviderAccount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:push_log_filter?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE push_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		app_id TEXT NOT NULL,
		provider_account_id INTEGER NOT NULL,
		provider_msg_id TEXT,
		request_data TEXT,
		response_data TEXT,
		status TEXT NOT NULL,
		error_message TEXT,
		cost_time INTEGER,
		created_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}

	now := time.Now()
	logs := []*model.PushLog{
		{TaskID: "task-a", AppID: "app", ProviderAccountID: 10, Status: "failed", CreatedAt: now},
		{TaskID: "task-b", AppID: "app", ProviderAccountID: 20, Status: "success", CreatedAt: now},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatalf("create logs: %v", err)
	}

	items, total, err := (&PushLogDAO{db: db}).List(&dto.LogListRequest{
		Page:       1,
		PageSize:   20,
		ProviderID: 20,
	})
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ProviderAccountID != 20 {
		t.Fatalf("unexpected provider filter result: total=%d items=%+v", total, items)
	}
}
