package service

import (
	"path/filepath"
	"testing"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAdminLogAndCallbackResponsesUseRFC3339UTC(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "admin-time-response.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE applications (id INTEGER PRIMARY KEY, app_id TEXT, app_secret TEXT, app_name TEXT, status INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_accounts (id INTEGER PRIMARY KEY, account_code TEXT, account_name TEXT, provider_code TEXT, provider_type TEXT, config TEXT, status INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE push_logs (id INTEGER PRIMARY KEY, task_id TEXT, app_id TEXT, provider_account_id INTEGER, provider_msg_id TEXT, request_data TEXT, response_data TEXT, status TEXT, error_message TEXT, cost_time INTEGER, created_at DATETIME)`,
		`CREATE TABLE callback_logs (id INTEGER PRIMARY KEY, type TEXT, task_id TEXT, app_id TEXT, provider_code TEXT, provider_id TEXT, mobile TEXT, content TEXT, callback_status TEXT, error_code TEXT, error_message TEXT, raw_data TEXT, created_at DATETIME)`,
		`INSERT INTO applications (id, app_id, app_secret, app_name, status) VALUES (1, 'utc-app', 'secret', 'UTC App', 1)`,
		`INSERT INTO provider_accounts (id, account_code, account_name, provider_code, provider_type, config, status) VALUES (1, 'utc-provider', 'UTC Provider', 'smtp', 'email', '{}', 1)`,
		`INSERT INTO push_logs (id, task_id, app_id, provider_account_id, status, created_at) VALUES (1, 'utc-task', 'utc-app', 1, 'success', '2026-08-04 18:00:00+08:00')`,
		`INSERT INTO callback_logs (id, type, task_id, app_id, provider_code, callback_status, created_at) VALUES (1, 'report', 'utc-task', 'utc-app', 'smtp', 'delivered', '2026-08-04 18:05:00+08:00')`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare time response schema: %v", err)
		}
	}

	appDAO := dao.NewApplicationDAOWithDB(db)
	logService := &AdminLogService{
		logDAO:             dao.NewPushLogDAOWithDB(db),
		appDAO:             appDAO,
		providerAccountDAO: dao.NewProviderAccountDAOWithDB(db),
	}
	logs, err := logService.GetLogList(&dto.LogListRequest{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("get log list: %v", err)
	}
	if len(logs.Items) != 1 || logs.Items[0].CreatedAt != "2026-08-04T10:00:00Z" {
		t.Fatalf("log response time = %+v", logs.Items)
	}

	callbackService := &AdminCallbackService{
		callbackLogDAO: dao.NewCallbackLogDAOWithDB(db),
		appDAO:         appDAO,
	}
	callbacks, err := callbackService.GetCallbackList(&dto.CallbackListRequest{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("get callback list: %v", err)
	}
	if len(callbacks.Items) != 1 || callbacks.Items[0].CreatedAt != "2026-08-04T10:05:00Z" {
		t.Fatalf("callback response time = %+v", callbacks.Items)
	}
}
