package service

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newAdminStatisticsTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "statistics.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		`CREATE TABLE applications (id INTEGER PRIMARY KEY AUTOINCREMENT, app_id TEXT NOT NULL, app_secret TEXT NOT NULL, app_name TEXT NOT NULL, status INTEGER NOT NULL, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE channels (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, type TEXT NOT NULL, message_template_id INTEGER NOT NULL DEFAULT 0, status INTEGER NOT NULL, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_accounts (id INTEGER PRIMARY KEY AUTOINCREMENT, status INTEGER NOT NULL, deleted_at DATETIME)`,
		`CREATE TABLE push_tasks (id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, app_id TEXT NOT NULL, channel_id INTEGER NOT NULL, provider_account_id INTEGER, message_type TEXT NOT NULL, receiver TEXT NOT NULL, status TEXT NOT NULL, created_at DATETIME, updated_at DATETIME)`,
		`CREATE TABLE push_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL, app_id TEXT NOT NULL, provider_account_id INTEGER NOT NULL, status TEXT NOT NULL, created_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	return db
}

func TestAdminStatisticsCountsTasksInsteadOfProviderAttempts(t *testing.T) {
	db := newAdminStatisticsTestDB(t)
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
		{TaskID: "task-a-processing", AppID: appA.AppID, ChannelID: channelA.ID, MessageType: "sms", Receiver: "13800138003", Status: "processing", CreatedAt: now},
		{TaskID: "task-a-sent", AppID: appA.AppID, ChannelID: channelA.ID, MessageType: "sms", Receiver: "13800138004", Status: "sent", CreatedAt: now},
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
		StartDate:   date,
		EndDate:     date,
		AppID:       appA.ID,
		ChannelID:   channelA.ID,
		MessageType: "sms",
	})
	if err != nil {
		t.Fatalf("get statistics: %v", err)
	}
	if stats.Summary.TotalCount != 5 || stats.Summary.SuccessCount != 1 || stats.Summary.FailureCount != 1 {
		t.Fatalf("unexpected summary: %+v", stats.Summary)
	}
	if stats.Summary.PendingCount != 1 || stats.Summary.ProcessingCount != 1 || stats.Summary.SentCount != 1 || stats.Summary.InProgressCount != 3 {
		t.Fatalf("unexpected in-progress summary: %+v", stats.Summary)
	}
	if stats.Summary.CompletedSuccessRate == nil || *stats.Summary.CompletedSuccessRate != 50 {
		t.Fatalf("completed success rate = %v, want 50", stats.Summary.CompletedSuccessRate)
	}
	if stats.Summary.SuccessRate != "20.00%" {
		t.Fatalf("legacy success rate = %q, want 20.00%%", stats.Summary.SuccessRate)
	}
	if stats.Period.StartDate != date || stats.Period.EndDate != date || stats.Period.Granularity != "day" {
		t.Fatalf("unexpected period: %+v", stats.Period)
	}
	if len(stats.Daily) != 1 || stats.Daily[0].Date != date || stats.Daily[0].TotalCount != 5 || stats.Daily[0].FailureCount != 1 || stats.Daily[0].InProgressCount != 3 {
		t.Fatalf("unexpected daily statistics: %+v", stats.Daily)
	}
	if len(stats.MessageTypeDistribution) != 1 || stats.MessageTypeDistribution[0].MessageType != "sms" || stats.MessageTypeDistribution[0].TotalCount != 5 {
		t.Fatalf("unexpected message type distribution: %+v", stats.MessageTypeDistribution)
	}
	if len(stats.TopApplications) != 1 || stats.TopApplications[0].AppID != appA.AppID || stats.TopApplications[0].AppName != appA.AppName || stats.TopApplications[0].TotalCount != 5 {
		t.Fatalf("unexpected filtered top applications: %+v", stats.TopApplications)
	}
	if len(stats.TopChannels) != 1 || stats.TopChannels[0].ChannelID != channelA.ID || stats.TopChannels[0].ChannelName != channelA.Name || stats.TopChannels[0].TotalCount != 5 {
		t.Fatalf("unexpected filtered top channels: %+v", stats.TopChannels)
	}

	allUserMessages, err := statisticsService.GetStatistics(&dto.StatisticsRequest{
		StartDate: date,
		EndDate:   date,
	})
	if err != nil {
		t.Fatalf("get all-user-message statistics: %v", err)
	}
	if allUserMessages.Summary.TotalCount != 6 {
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
	if empty.Summary.TotalCount != 0 || len(empty.Daily) != 1 || empty.Daily[0].Date != date || empty.Daily[0].TotalCount != 0 {
		t.Fatalf("missing application filter must return no tasks: %+v", empty)
	}
	if empty.Summary.CompletedSuccessRate != nil {
		t.Fatalf("empty completed success rate = %v, want nil", empty.Summary.CompletedSuccessRate)
	}

	dashboard, err := statisticsService.GetDashboard()
	if err != nil {
		t.Fatalf("get dashboard: %v", err)
	}
	if dashboard.TodayPushCount != 6 || dashboard.TodaySuccessCount != 2 || dashboard.TodayFailedCount != 1 || dashboard.TodayInProgressCount != 3 {
		t.Fatalf("unexpected dashboard task counts: %+v", dashboard)
	}
	if dashboard.TodayCompletedSuccessRate == nil || *dashboard.TodayCompletedSuccessRate != 66.67 {
		t.Fatalf("dashboard completed success rate = %v, want 66.67", dashboard.TodayCompletedSuccessRate)
	}
	if dashboard.TotalPushCount != 6 {
		t.Fatalf("total push count = %d, want 6", dashboard.TotalPushCount)
	}

	top, err := statisticsService.GetTopApplications(10)
	if err != nil {
		t.Fatalf("get top applications: %v", err)
	}
	if len(top) != 2 || top[0].AppID != appA.AppID || top[0].PushCount != 5 {
		t.Fatalf("unexpected top applications: %+v", top)
	}

	recent, err := statisticsService.GetRecentActivities(10)
	if err != nil {
		t.Fatalf("get recent activities: %v", err)
	}
	if len(recent) != 6 {
		t.Fatalf("recent activities include admin test tasks: %+v", recent)
	}
	for _, activity := range recent {
		if activity.AppName == "未知应用" {
			t.Fatalf("admin test appeared as an unknown application: %+v", recent)
		}
	}
}

func TestAdminStatisticsAppliesEachDimensionFilter(t *testing.T) {
	db := newAdminStatisticsTestDB(t)
	statisticsService := &AdminStatisticsService{db: db}
	now := time.Now()
	date := now.Format(statisticsDateLayout)

	appA := &model.Application{AppID: "filter-app-a", AppSecret: "secret", AppName: "筛选应用 A", Status: 1}
	appB := &model.Application{AppID: "filter-app-b", AppSecret: "secret", AppName: "筛选应用 B", Status: 1}
	if err := db.Select("app_id", "app_secret", "app_name", "status", "created_at", "updated_at").Create([]*model.Application{appA, appB}).Error; err != nil {
		t.Fatalf("create applications: %v", err)
	}
	channelA := &model.Channel{Name: "筛选通道 A", Type: "sms", Status: 1}
	channelB := &model.Channel{Name: "筛选通道 B", Type: "sms", Status: 1}
	if err := db.Select("name", "type", "status", "created_at", "updated_at").Create([]*model.Channel{channelA, channelB}).Error; err != nil {
		t.Fatalf("create channels: %v", err)
	}

	tasks := []*model.PushTask{
		{TaskID: "filter-base", AppID: appA.AppID, ChannelID: channelA.ID, MessageType: "sms", Receiver: "13800138000", Status: "success", CreatedAt: now},
		{TaskID: "filter-other-app", AppID: appB.AppID, ChannelID: channelA.ID, MessageType: "sms", Receiver: "13800138001", Status: "success", CreatedAt: now},
		{TaskID: "filter-other-channel", AppID: appA.AppID, ChannelID: channelB.ID, MessageType: "sms", Receiver: "13800138002", Status: "success", CreatedAt: now},
		{TaskID: "filter-other-type", AppID: appA.AppID, ChannelID: channelA.ID, MessageType: "email", Receiver: "user@example.com", Status: "success", CreatedAt: now},
	}
	if err := db.Select("task_id", "app_id", "channel_id", "message_type", "receiver", "status", "created_at", "updated_at").Create(&tasks).Error; err != nil {
		t.Fatalf("create push tasks: %v", err)
	}

	tests := []struct {
		name string
		req  *dto.StatisticsRequest
		want int64
	}{
		{
			name: "application",
			req:  &dto.StatisticsRequest{StartDate: date, EndDate: date, AppID: appA.ID},
			want: 3,
		},
		{
			name: "channel",
			req:  &dto.StatisticsRequest{StartDate: date, EndDate: date, ChannelID: channelA.ID},
			want: 3,
		},
		{
			name: "message type",
			req:  &dto.StatisticsRequest{StartDate: date, EndDate: date, MessageType: "sms"},
			want: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stats, err := statisticsService.GetStatistics(test.req)
			if err != nil {
				t.Fatalf("GetStatistics() error = %v", err)
			}
			if stats.Summary.TotalCount != test.want {
				t.Fatalf("total count = %d, want %d", stats.Summary.TotalCount, test.want)
			}
		})
	}
}

func TestAdminStatisticsUsesInclusiveDateBoundaries(t *testing.T) {
	db := newAdminStatisticsTestDB(t)
	statisticsService := &AdminStatisticsService{db: db}
	start, rangeEnd, err := statisticsDateRange("2026-07-20", "2026-07-21")
	if err != nil {
		t.Fatalf("statisticsDateRange() error = %v", err)
	}

	tasks := []*model.PushTask{
		{TaskID: "before-range", AppID: "boundary-app", ChannelID: 1, MessageType: "sms", Receiver: "13800138000", Status: "success", CreatedAt: start.Add(-time.Nanosecond)},
		{TaskID: "at-range-start", AppID: "boundary-app", ChannelID: 1, MessageType: "sms", Receiver: "13800138001", Status: "success", CreatedAt: start},
		{TaskID: "at-range-end", AppID: "boundary-app", ChannelID: 1, MessageType: "sms", Receiver: "13800138002", Status: "failed", CreatedAt: rangeEnd.Add(-time.Nanosecond)},
		{TaskID: "after-range", AppID: "boundary-app", ChannelID: 1, MessageType: "sms", Receiver: "13800138003", Status: "success", CreatedAt: rangeEnd},
	}
	if err := db.Select("task_id", "app_id", "channel_id", "message_type", "receiver", "status", "created_at", "updated_at").Create(&tasks).Error; err != nil {
		t.Fatalf("create push tasks: %v", err)
	}

	stats, err := statisticsService.GetStatistics(&dto.StatisticsRequest{
		StartDate: "2026-07-20",
		EndDate:   "2026-07-21",
	})
	if err != nil {
		t.Fatalf("GetStatistics() error = %v", err)
	}
	if stats.Summary.TotalCount != 2 || stats.Summary.SuccessCount != 1 || stats.Summary.FailureCount != 1 {
		t.Fatalf("unexpected boundary summary: %+v", stats.Summary)
	}
	if len(stats.Daily) != 2 || stats.Daily[0].TotalCount != 1 || stats.Daily[1].TotalCount != 1 {
		t.Fatalf("unexpected boundary daily buckets: %+v", stats.Daily)
	}
}

func TestStatisticsDateRangesUseShanghaiCalendarDaysInUTC(t *testing.T) {
	start, end, err := statisticsDateRange("2026-07-23", "2026-07-23")
	if err != nil {
		t.Fatalf("statistics date range: %v", err)
	}
	if got, want := start.Format(time.RFC3339), "2026-07-22T16:00:00Z"; got != want {
		t.Fatalf("start = %s, want %s", got, want)
	}
	if got, want := end.Format(time.RFC3339), "2026-07-23T16:00:00Z"; got != want {
		t.Fatalf("end = %s, want %s", got, want)
	}

	todayStart, tomorrowStart := localDayRange(time.Date(2026, 7, 23, 15, 59, 59, 0, time.UTC))
	if !todayStart.Equal(start) || !tomorrowStart.Equal(end) {
		t.Fatalf("business day range = [%s, %s), want [%s, %s)", todayStart, tomorrowStart, start, end)
	}
}

func TestStatisticsDateBucketExpressionsReturnTextForEveryDialect(t *testing.T) {
	tests := []struct {
		dialect string
		want    string
	}{
		{
			dialect: "sqlite",
			want:    "strftime('%Y-%m-%d', created_at, '+8 hours')",
		},
		{
			dialect: "mysql",
			want:    "DATE_FORMAT(CONVERT_TZ(created_at, '+00:00', '+08:00'), '%Y-%m-%d')",
		},
		{
			dialect: "postgres",
			want:    "TO_CHAR(created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')",
		},
	}

	for _, test := range tests {
		t.Run(test.dialect, func(t *testing.T) {
			if got := statisticsDateBucketExpression(test.dialect); got != test.want {
				t.Fatalf("statisticsDateBucketExpression(%q) = %q, want %q", test.dialect, got, test.want)
			}
		})
	}
}

func TestStatisticsTimezoneLabelIsBusinessTimezone(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	if got, want := statisticsTimezoneLabel(now), "Asia/Shanghai"; got != want {
		t.Fatalf("statistics timezone = %q, want %q", got, want)
	}
}

func TestAdminStatisticsValidatesRangeAndMessageType(t *testing.T) {
	db := newAdminStatisticsTestDB(t)
	statisticsService := &AdminStatisticsService{db: db}
	today := time.Now().In(time.Local)

	tests := []struct {
		name string
		req  *dto.StatisticsRequest
	}{
		{
			name: "nil request",
			req:  nil,
		},
		{
			name: "start date only",
			req:  &dto.StatisticsRequest{StartDate: today.Format(statisticsDateLayout)},
		},
		{
			name: "end date only",
			req:  &dto.StatisticsRequest{EndDate: today.Format(statisticsDateLayout)},
		},
		{
			name: "invalid date",
			req: &dto.StatisticsRequest{
				StartDate: "2026-02-30",
				EndDate:   "2026-03-01",
			},
		},
		{
			name: "reversed range",
			req: &dto.StatisticsRequest{
				StartDate: "2026-03-02",
				EndDate:   "2026-03-01",
			},
		},
		{
			name: "ninety one days",
			req: &dto.StatisticsRequest{
				StartDate: today.AddDate(0, 0, -90).Format(statisticsDateLayout),
				EndDate:   today.Format(statisticsDateLayout),
			},
		},
		{
			name: "invalid message type",
			req: &dto.StatisticsRequest{
				StartDate:   today.Format(statisticsDateLayout),
				EndDate:     today.Format(statisticsDateLayout),
				MessageType: "fax",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := statisticsService.GetStatistics(test.req)
			if !errors.Is(err, ErrInvalidStatisticsRequest) {
				t.Fatalf("GetStatistics() error = %v, want ErrInvalidStatisticsRequest", err)
			}
		})
	}

	ninetyDays, err := statisticsService.GetStatistics(&dto.StatisticsRequest{
		StartDate: today.AddDate(0, 0, -89).Format(statisticsDateLayout),
		EndDate:   today.Format(statisticsDateLayout),
	})
	if err != nil {
		t.Fatalf("90-day statistics range: %v", err)
	}
	if len(ninetyDays.Daily) != 90 {
		t.Fatalf("90-day daily buckets = %d, want 90", len(ninetyDays.Daily))
	}

	defaultRange, err := statisticsService.GetStatistics(&dto.StatisticsRequest{})
	if err != nil {
		t.Fatalf("default statistics range: %v", err)
	}
	wantStartDate := today.AddDate(0, 0, -29).Format(statisticsDateLayout)
	if defaultRange.Period.StartDate != wantStartDate || defaultRange.Period.EndDate != today.Format(statisticsDateLayout) {
		t.Fatalf("default period = %+v, want %s through %s", defaultRange.Period, wantStartDate, today.Format(statisticsDateLayout))
	}
	if len(defaultRange.Daily) != 30 {
		t.Fatalf("default daily buckets = %d, want 30", len(defaultRange.Daily))
	}
	if defaultRange.Daily[0].TotalCount != 0 || defaultRange.Summary.CompletedSuccessRate != nil {
		t.Fatalf("unexpected empty statistics response: %+v", defaultRange)
	}
	if defaultRange.MessageTypeDistribution == nil || defaultRange.TopApplications == nil || defaultRange.TopChannels == nil {
		t.Fatalf("empty response arrays must be non-nil: %+v", defaultRange)
	}
}

func TestAdminStatisticsFillsMissingCalendarDays(t *testing.T) {
	db := newAdminStatisticsTestDB(t)
	statisticsService := &AdminStatisticsService{db: db}
	firstDay := time.Date(2026, 7, 20, 12, 0, 0, 0, time.Local)
	lastDay := firstDay.AddDate(0, 0, 2)

	tasks := []*model.PushTask{
		{TaskID: "first-day", AppID: "app-a", ChannelID: 1, MessageType: "sms", Receiver: "13800138000", Status: "success", CreatedAt: firstDay},
		{TaskID: "last-day", AppID: "app-a", ChannelID: 1, MessageType: "sms", Receiver: "13800138001", Status: "failed", CreatedAt: lastDay},
	}
	if err := db.Select("task_id", "app_id", "channel_id", "message_type", "receiver", "status", "created_at", "updated_at").Create(&tasks).Error; err != nil {
		t.Fatalf("create push tasks: %v", err)
	}

	stats, err := statisticsService.GetStatistics(&dto.StatisticsRequest{
		StartDate: firstDay.Format(statisticsDateLayout),
		EndDate:   lastDay.Format(statisticsDateLayout),
	})
	if err != nil {
		t.Fatalf("get statistics: %v", err)
	}
	if len(stats.Daily) != 3 {
		t.Fatalf("daily buckets = %d, want 3", len(stats.Daily))
	}
	if stats.Daily[0].TotalCount != 1 || stats.Daily[1].Date != "2026-07-21" || stats.Daily[1].TotalCount != 0 || stats.Daily[2].TotalCount != 1 {
		t.Fatalf("missing date was not zero-filled: %+v", stats.Daily)
	}
	if stats.Daily[1].CompletedSuccessRate != nil || stats.Daily[1].SuccessRate != "0.00%" {
		t.Fatalf("unexpected empty-day rates: %+v", stats.Daily[1])
	}
}

func TestAdminStatisticsKeepsBucketsForDeletedResources(t *testing.T) {
	db := newAdminStatisticsTestDB(t)
	statisticsService := &AdminStatisticsService{db: db}
	date := time.Now().Format(statisticsDateLayout)

	task := &model.PushTask{
		TaskID:      "deleted-resources",
		AppID:       "deleted-app",
		ChannelID:   404,
		MessageType: "sms",
		Receiver:    "13800138000",
		Status:      "success",
		CreatedAt:   time.Now(),
	}
	if err := db.Select("task_id", "app_id", "channel_id", "message_type", "receiver", "status", "created_at", "updated_at").Create(task).Error; err != nil {
		t.Fatalf("create push task: %v", err)
	}

	stats, err := statisticsService.GetStatistics(&dto.StatisticsRequest{StartDate: date, EndDate: date})
	if err != nil {
		t.Fatalf("get statistics: %v", err)
	}
	if len(stats.TopApplications) != 1 || stats.TopApplications[0].AppID != "deleted-app" || stats.TopApplications[0].AppName != "已删除应用 (deleted-app)" {
		t.Fatalf("deleted application bucket lost: %+v", stats.TopApplications)
	}
	if len(stats.TopChannels) != 1 || stats.TopChannels[0].ChannelID != 404 || stats.TopChannels[0].ChannelName != "已删除通道 (#404)" {
		t.Fatalf("deleted channel bucket lost: %+v", stats.TopChannels)
	}
}

func TestAdminStatisticsPropagatesDatabaseErrors(t *testing.T) {
	db := newAdminStatisticsTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql database: %v", err)
	}

	statisticsService := &AdminStatisticsService{db: db}
	date := time.Now().Format(statisticsDateLayout)
	if _, err := statisticsService.GetStatistics(&dto.StatisticsRequest{StartDate: date, EndDate: date}); err == nil {
		t.Fatal("GetStatistics() error = nil after database close")
	}
	if _, err := statisticsService.GetDashboard(); err == nil {
		t.Fatal("GetDashboard() error = nil after database close")
	}
	if _, err := statisticsService.GetTopApplications(10); err == nil {
		t.Fatal("GetTopApplications() error = nil after database close")
	}
	if _, err := statisticsService.GetRecentActivities(10); err == nil {
		t.Fatal("GetRecentActivities() error = nil after database close")
	}
}
