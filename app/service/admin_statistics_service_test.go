package service

import (
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAdminStatisticsCountsTasksInsteadOfProviderAttempts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin_statistics?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE applications (id INTEGER PRIMARY KEY AUTOINCREMENT, app_id TEXT NOT NULL, app_secret TEXT NOT NULL, app_name TEXT NOT NULL, status INTEGER NOT NULL, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE channels (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, type TEXT NOT NULL, message_template_id INTEGER NOT NULL DEFAULT 0, status INTEGER NOT NULL, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_accounts (id INTEGER PRIMARY KEY AUTOINCREMENT, status INTEGER NOT NULL, deleted_at DATETIME)`,
		`CREATE TABLE push_tasks (id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, app_id TEXT NOT NULL, channel_id INTEGER NOT NULL, message_type TEXT NOT NULL, receiver TEXT NOT NULL, status TEXT NOT NULL, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE push_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, app_id TEXT NOT NULL, provider_account_id INTEGER NOT NULL, status TEXT NOT NULL, created_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	appA := &model.Application{AppID: "app-a", AppSecret: "secret", AppName: "应用 A", Status: 1}
	appB := &model.Application{AppID: "app-b", AppSecret: "secret", AppName: "应用 B", Status: 1}
	if err := db.Select("app_id", "app_secret", "app_name", "status", "created_at", "updated_at").Create([]*model.Application{appA, appB}).Error; err != nil {
		t.Fatalf("create applications: %v", err)
	}
	channelA := &model.Channel{Name: "短信通道", Type: "sms", Status: 1}
	channelB := &model.Channel{Name: "邮件通道", Type: "email", Status: 1}
	if err := db.Select("name", "type", "message_template_id", "status", "created_at", "updated_at").Create([]*model.Channel{channelA, channelB}).Error; err != nil {
		t.Fatalf("create channels: %v", err)
	}

	now := time.Now()
	tasks := []*model.PushTask{
		{TaskID: "task-a-success", AppID: appA.AppID, ChannelID: channelA.ID, MessageType: "sms", Receiver: "13800138000", Status: "success", CreatedAt: now},
		{TaskID: "task-a-failed", AppID: appA.AppID, ChannelID: channelA.ID, MessageType: "sms", Receiver: "13800138001", Status: "failed", CreatedAt: now},
		{TaskID: "task-a-pending", AppID: appA.AppID, ChannelID: channelA.ID, MessageType: "sms", Receiver: "13800138002", Status: "pending", CreatedAt: now},
		{TaskID: "task-b-success", AppID: appB.AppID, ChannelID: channelB.ID, MessageType: "email", Receiver: "user@example.com", Status: "success", CreatedAt: now},
		{TaskID: "task-admin-test", AppID: adminTestAppID, ChannelID: channelA.ID, MessageType: "sms", Receiver: "13800138999", Status: "success", CreatedAt: now},
	}
	if err := db.Select("task_id", "app_id", "channel_id", "message_type", "receiver", "status", "created_at", "updated_at").Create(&tasks).Error; err != nil {
		t.Fatalf("create push tasks: %v", err)
	}

	// task-a-failed was attempted three times. These are provider attempts, not
	// three additional user messages, and therefore must not inflate statistics.
	logs := []*model.PushLog{
		{TaskID: "task-a-failed", AppID: appA.AppID, ProviderAccountID: 1, Status: "failed", CreatedAt: now},
		{TaskID: "task-a-failed", AppID: appA.AppID, ProviderAccountID: 2, Status: "failed", CreatedAt: now},
		{TaskID: "task-a-failed", AppID: appA.AppID, ProviderAccountID: 3, Status: "failed", CreatedAt: now},
	}
	if err := db.Select("task_id", "app_id", "provider_account_id", "status", "created_at").Create(&logs).Error; err != nil {
		t.Fatalf("create push logs: %v", err)
	}

	date := now.Format("2006-01-02")
	statisticsService := &AdminStatisticsService{db: db}
	stats, err := statisticsService.GetStatistics(&dto.StatisticsRequest{
		StartDate: date,
		EndDate:   date,
		AppID:     appA.ID,
		ChannelID: channelA.ID,
	})
	if err != nil {
		t.Fatalf("get statistics: %v", err)
	}
	if stats.Summary.TotalCount != 3 || stats.Summary.SuccessCount != 1 || stats.Summary.FailureCount != 1 {
		t.Fatalf("unexpected summary: %+v", stats.Summary)
	}
	if len(stats.Daily) != 1 || stats.Daily[0].TotalCount != 3 || stats.Daily[0].FailureCount != 1 {
		t.Fatalf("unexpected daily statistics: %+v", stats.Daily)
	}

	allUserMessages, err := statisticsService.GetStatistics(&dto.StatisticsRequest{
		StartDate: date,
		EndDate:   date,
	})
	if err != nil {
		t.Fatalf("get all-user-message statistics: %v", err)
	}
	if allUserMessages.Summary.TotalCount != 4 {
		t.Fatalf("admin test polluted user-message statistics: %+v", allUserMessages.Summary)
	}

	empty, err := statisticsService.GetStatistics(&dto.StatisticsRequest{
		StartDate: date,
		EndDate:   date,
		AppID:     999_999,
	})
	if err != nil {
		t.Fatalf("get statistics for missing application: %v", err)
	}
	if empty.Summary.TotalCount != 0 || len(empty.Daily) != 0 {
		t.Fatalf("missing application filter must return no tasks: %+v", empty)
	}

	dashboard, err := statisticsService.GetDashboard()
	if err != nil {
		t.Fatalf("get dashboard: %v", err)
	}
	if dashboard.TodayPushCount != 4 || dashboard.TodaySuccessCount != 2 || dashboard.TodayFailedCount != 1 {
		t.Fatalf("unexpected dashboard task counts: %+v", dashboard)
	}
	if dashboard.TotalPushCount != 4 {
		t.Fatalf("total push count = %d, want 4", dashboard.TotalPushCount)
	}

	top, err := statisticsService.GetTopApplications(10)
	if err != nil {
		t.Fatalf("get top applications: %v", err)
	}
	if len(top) != 2 || top[0].AppID != appA.AppID || top[0].PushCount != 3 {
		t.Fatalf("unexpected top applications: %+v", top)
	}

	recent, err := statisticsService.GetRecentActivities(10)
	if err != nil {
		t.Fatalf("get recent activities: %v", err)
	}
	if len(recent) != 4 {
		t.Fatalf("recent activities include admin test tasks: %+v", recent)
	}
	for _, activity := range recent {
		if activity.AppName == "未知应用" {
			t.Fatalf("admin test appeared as an unknown application: %+v", recent)
		}
	}
}
