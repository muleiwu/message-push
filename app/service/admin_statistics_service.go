package service

import (
	"errors"
	"fmt"
	"math"
	"time"

	"gorm.io/gorm"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/timeutil"
)

// AdminStatisticsService 统计分析服务
type AdminStatisticsService struct {
	db *gorm.DB
}

func (s *AdminStatisticsService) userTaskQuery() *gorm.DB {
	return s.db.Model(&model.PushTask{}).Where("app_id <> ?", adminTestAppID)
}

const (
	statisticsDateLayout       = "2006-01-02"
	statisticsDefaultRangeDays = 30
	statisticsMaxRangeDays     = 90
	statisticsTopLimit         = 10
)

// ErrInvalidStatisticsRequest marks validation errors that should be exposed as
// HTTP 400 by the admin statistics controller.
var ErrInvalidStatisticsRequest = errors.New("invalid statistics request")

type statisticsRange struct {
	start     time.Time
	end       time.Time
	startDate string
	endDate   string
	timezone  string
}

type statisticsAggregateRow struct {
	TotalCount      int64
	SuccessCount    int64
	FailureCount    int64
	PendingCount    int64
	ProcessingCount int64
	SentCount       int64
}

const statisticsAggregateSelect = `
	COUNT(*) AS total_count,
	COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) AS success_count,
	COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) AS failure_count,
	COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) AS pending_count,
	COALESCE(SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END), 0) AS processing_count,
	COALESCE(SUM(CASE WHEN status = 'sent' THEN 1 ELSE 0 END), 0) AS sent_count`

// statisticsDateRange converts inclusive Shanghai business dates into a
// half-open UTC range. Comparing instants keeps the created_at index usable.
func statisticsDateRange(startDate, endDate string) (time.Time, time.Time, error) {
	start, end, err := timeutil.BusinessDateRangeUTC(startDate, endDate)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return start, end, nil
}

func resolveStatisticsRange(req *dto.StatisticsRequest, now time.Time) (statisticsRange, error) {
	if req == nil {
		return statisticsRange{}, fmt.Errorf("%w: request is required", ErrInvalidStatisticsRequest)
	}

	startDate, endDate := req.StartDate, req.EndDate
	switch {
	case startDate == "" && endDate == "":
		today := timeutil.BusinessDayStart(now)
		startDate = today.AddDate(0, 0, -(statisticsDefaultRangeDays - 1)).Format(statisticsDateLayout)
		endDate = today.Format(statisticsDateLayout)
	case startDate == "" || endDate == "":
		return statisticsRange{}, fmt.Errorf("%w: start_date and end_date must be provided together", ErrInvalidStatisticsRequest)
	}

	start, end, err := statisticsDateRange(startDate, endDate)
	if err != nil {
		return statisticsRange{}, fmt.Errorf("%w: %v", ErrInvalidStatisticsRequest, err)
	}
	lastAllowedDay := start.AddDate(0, 0, statisticsMaxRangeDays-1)
	if end.AddDate(0, 0, -1).After(lastAllowedDay) {
		return statisticsRange{}, fmt.Errorf("%w: date range cannot exceed %d days", ErrInvalidStatisticsRequest, statisticsMaxRangeDays)
	}
	if req.MessageType != "" && !constants.IsValidMessageType(req.MessageType) {
		return statisticsRange{}, fmt.Errorf("%w: unsupported message_type %q", ErrInvalidStatisticsRequest, req.MessageType)
	}

	return statisticsRange{
		start:     start,
		end:       end,
		startDate: startDate,
		endDate:   endDate,
		timezone:  statisticsTimezoneLabel(now),
	}, nil
}

func localDayRange(now time.Time) (time.Time, time.Time) {
	start := timeutil.BusinessDayStart(now)
	return start.UTC(), start.AddDate(0, 0, 1).UTC()
}

func completedSuccessRate(success, failure int64) *float64 {
	completed := success + failure
	if completed == 0 {
		return nil
	}
	rate := math.Round(float64(success)/float64(completed)*10000) / 100
	return &rate
}

func legacySuccessRate(success, total int64) string {
	if total == 0 {
		return "0.00%"
	}
	return fmt.Sprintf("%.2f%%", float64(success)/float64(total)*100)
}

func statisticsCounts(row statisticsAggregateRow) dto.StatisticsCounts {
	return dto.StatisticsCounts{
		TotalCount:           row.TotalCount,
		SuccessCount:         row.SuccessCount,
		FailureCount:         row.FailureCount,
		PendingCount:         row.PendingCount,
		ProcessingCount:      row.ProcessingCount,
		SentCount:            row.SentCount,
		InProgressCount:      row.PendingCount + row.ProcessingCount + row.SentCount,
		CompletedSuccessRate: completedSuccessRate(row.SuccessCount, row.FailureCount),
	}
}

// statisticsDateBucketExpression groups UTC instants by the fixed Shanghai
// business day without depending on the database session timezone.
func statisticsDateBucketExpression(dialect string) string {
	switch dialect {
	case "sqlite":
		return "strftime('%Y-%m-%d', created_at, '+8 hours')"
	case "mysql":
		return "DATE_FORMAT(CONVERT_TZ(created_at, '+00:00', '+08:00'), '%Y-%m-%d')"
	case "postgres":
		return "TO_CHAR(created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')"
	default:
		return "CAST(DATE(created_at) AS CHAR)"
	}
}

func statisticsTimezoneLabel(_ time.Time) string {
	return timeutil.BusinessTimeZone
}

// NewAdminStatisticsService 创建统计分析服务实例
func NewAdminStatisticsService() *AdminStatisticsService {
	return &AdminStatisticsService{db: helper.GetDatabase()}
}

// GetStatistics 获取推送统计
func (s *AdminStatisticsService) GetStatistics(req *dto.StatisticsRequest) (*dto.StatisticsResponse, error) {
	db := s.db
	dateRange, err := resolveStatisticsRange(req, timeutil.Now())
	if err != nil {
		return nil, err
	}

	// 所有聚合都从同一个基础查询克隆，确保筛选口径完全一致。
	dialect := db.Dialector.Name()
	query := s.userTaskQuery().Where("created_at >= ? AND created_at < ?", dateRange.start, dateRange.end)

	if req.AppID > 0 {
		var app model.Application
		if err := db.Unscoped().First(&app, req.AppID).Error; err == nil {
			query = query.Where("app_id = ?", app.AppID)
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			query = query.Where("1 = 0")
		} else {
			return nil, fmt.Errorf("failed to resolve application filter: %w", err)
		}
	}
	if req.ChannelID > 0 {
		query = query.Where("channel_id = ?", req.ChannelID)
	}
	if req.MessageType != "" {
		query = query.Where("message_type = ?", req.MessageType)
	}

	var summaryRow statisticsAggregateRow
	if err := query.Session(&gorm.Session{}).
		Select(statisticsAggregateSelect).
		Scan(&summaryRow).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate statistics summary: %w", err)
	}

	var dailyRows []struct {
		Date      string
		Aggregate statisticsAggregateRow `gorm:"embedded"`
	}

	dateExpression := statisticsDateBucketExpression(dialect)
	if err := query.Session(&gorm.Session{}).
		Select(dateExpression + " AS date, " + statisticsAggregateSelect).
		Group(dateExpression).
		Order("date ASC").
		Scan(&dailyRows).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate daily statistics: %w", err)
	}

	var messageTypeRows []struct {
		MessageType string
		Aggregate   statisticsAggregateRow `gorm:"embedded"`
	}
	if err := query.Session(&gorm.Session{}).
		Select("message_type, " + statisticsAggregateSelect).
		Group("message_type").
		Order("total_count DESC, message_type ASC").
		Scan(&messageTypeRows).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate message type statistics: %w", err)
	}

	var applicationRows []struct {
		AppID     string
		Aggregate statisticsAggregateRow `gorm:"embedded"`
	}
	if err := query.Session(&gorm.Session{}).
		Select("app_id, " + statisticsAggregateSelect).
		Group("app_id").
		Order("total_count DESC, app_id ASC").
		Limit(statisticsTopLimit).
		Scan(&applicationRows).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate top applications: %w", err)
	}

	applicationIDs := make([]string, 0, len(applicationRows))
	for _, row := range applicationRows {
		applicationIDs = append(applicationIDs, row.AppID)
	}
	applicationsByAppID := make(map[string]model.Application, len(applicationIDs))
	if len(applicationIDs) > 0 {
		var applications []model.Application
		if err := db.Unscoped().Where("app_id IN ?", applicationIDs).Find(&applications).Error; err != nil {
			return nil, fmt.Errorf("failed to load top application names: %w", err)
		}
		for _, app := range applications {
			applicationsByAppID[app.AppID] = app
		}
	}

	var channelRows []struct {
		ChannelID uint
		Aggregate statisticsAggregateRow `gorm:"embedded"`
	}
	if err := query.Session(&gorm.Session{}).
		Select("channel_id, " + statisticsAggregateSelect).
		Group("channel_id").
		Order("total_count DESC, channel_id ASC").
		Limit(statisticsTopLimit).
		Scan(&channelRows).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate top channels: %w", err)
	}

	channelIDs := make([]uint, 0, len(channelRows))
	for _, row := range channelRows {
		channelIDs = append(channelIDs, row.ChannelID)
	}
	channelsByID := make(map[uint]model.Channel, len(channelIDs))
	if len(channelIDs) > 0 {
		var channels []model.Channel
		if err := db.Unscoped().Where("id IN ?", channelIDs).Find(&channels).Error; err != nil {
			return nil, fmt.Errorf("failed to load top channel names: %w", err)
		}
		for _, channel := range channels {
			channelsByID[channel.ID] = channel
		}
	}

	summaryCounts := statisticsCounts(summaryRow)
	response := &dto.StatisticsResponse{
		Period: dto.StatisticsPeriod{
			StartDate:   dateRange.startDate,
			EndDate:     dateRange.endDate,
			Timezone:    dateRange.timezone,
			Granularity: "day",
		},
		Summary: dto.StatisticsSummary{
			StatisticsCounts: summaryCounts,
			SuccessRate:      legacySuccessRate(summaryRow.SuccessCount, summaryRow.TotalCount),
		},
		Daily:                   make([]*dto.DailyStatistics, 0, statisticsMaxRangeDays),
		MessageTypeDistribution: make([]*dto.MessageTypeStatistics, 0, len(messageTypeRows)),
		TopApplications:         make([]*dto.ApplicationStatistics, 0, len(applicationRows)),
		TopChannels:             make([]*dto.ChannelStatistics, 0, len(channelRows)),
	}

	dailyByDate := make(map[string]statisticsAggregateRow, len(dailyRows))
	for _, row := range dailyRows {
		dailyByDate[row.Date] = row.Aggregate
	}
	startBusinessDay := dateRange.start.In(timeutil.BusinessLocation())
	endBusinessDay := dateRange.end.In(timeutil.BusinessLocation())
	for day := startBusinessDay; day.Before(endBusinessDay); day = day.AddDate(0, 0, 1) {
		date := day.Format(statisticsDateLayout)
		row := dailyByDate[date]
		response.Daily = append(response.Daily, &dto.DailyStatistics{
			Date: date,
			StatisticsSummary: dto.StatisticsSummary{
				StatisticsCounts: statisticsCounts(row),
				SuccessRate:      legacySuccessRate(row.SuccessCount, row.TotalCount),
			},
		})
	}

	for _, row := range messageTypeRows {
		response.MessageTypeDistribution = append(response.MessageTypeDistribution, &dto.MessageTypeStatistics{
			MessageType:      row.MessageType,
			StatisticsCounts: statisticsCounts(row.Aggregate),
		})
	}

	for _, row := range applicationRows {
		app, found := applicationsByAppID[row.AppID]
		item := &dto.ApplicationStatistics{
			AppID:            row.AppID,
			AppName:          fmt.Sprintf("已删除应用 (%s)", row.AppID),
			StatisticsCounts: statisticsCounts(row.Aggregate),
		}
		if found {
			item.ID = app.ID
			item.AppName = app.AppName
		}
		response.TopApplications = append(response.TopApplications, item)
	}

	for _, row := range channelRows {
		channel, found := channelsByID[row.ChannelID]
		item := &dto.ChannelStatistics{
			ChannelID:        row.ChannelID,
			ChannelName:      fmt.Sprintf("已删除通道 (#%d)", row.ChannelID),
			StatisticsCounts: statisticsCounts(row.Aggregate),
		}
		if found {
			item.ChannelName = channel.Name
			item.ChannelType = channel.Type
		}
		response.TopChannels = append(response.TopChannels, item)
	}

	return response, nil
}

// GetDashboard 获取仪表盘数据
func (s *AdminStatisticsService) GetDashboard() (*dto.DashboardResponse, error) {
	db := s.db
	resp := &dto.DashboardResponse{}

	// 1. 统计 Applications
	if err := db.Model(&model.Application{}).Count(&resp.TotalApplications).Error; err != nil {
		return nil, fmt.Errorf("failed to count applications: %w", err)
	}
	if err := db.Model(&model.Application{}).Where("status = ?", 1).Count(&resp.ActiveApplications).Error; err != nil {
		return nil, fmt.Errorf("failed to count active applications: %w", err)
	}

	// 2. 统计 Channels
	if err := db.Model(&model.Channel{}).Count(&resp.TotalChannels).Error; err != nil {
		return nil, fmt.Errorf("failed to count channels: %w", err)
	}
	if err := db.Model(&model.Channel{}).Where("status = ?", 1).Count(&resp.ActiveChannels).Error; err != nil {
		return nil, fmt.Errorf("failed to count active channels: %w", err)
	}

	// 3. 统计 Provider Accounts
	if err := db.Model(&model.ProviderAccount{}).Count(&resp.TotalProviders).Error; err != nil {
		return nil, fmt.Errorf("failed to count providers: %w", err)
	}
	if err := db.Model(&model.ProviderAccount{}).Where("status = ?", 1).Count(&resp.ActiveProviders).Error; err != nil {
		return nil, fmt.Errorf("failed to count active providers: %w", err)
	}

	// 4. 统计今日推送
	todayStart, tomorrowStart := localDayRange(timeutil.Now())

	var todayStats statisticsAggregateRow
	if err := s.userTaskQuery().
		Select(statisticsAggregateSelect).
		Where("created_at >= ? AND created_at < ?", todayStart, tomorrowStart).
		Scan(&todayStats).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate today's push statistics: %w", err)
	}

	resp.TodayPushCount = todayStats.TotalCount
	resp.TodaySuccessCount = todayStats.SuccessCount
	resp.TodayFailedCount = todayStats.FailureCount
	resp.TodayInProgressCount = todayStats.PendingCount + todayStats.ProcessingCount + todayStats.SentCount
	resp.TodaySuccessRate = legacySuccessRate(resp.TodaySuccessCount, resp.TodayPushCount)
	resp.TodayCompletedSuccessRate = completedSuccessRate(resp.TodaySuccessCount, resp.TodayFailedCount)

	// 5. 统计总推送量
	if err := s.userTaskQuery().Count(&resp.TotalPushCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count total pushes: %w", err)
	}

	return resp, nil
}

// GetTopApplications 获取热门应用
func (s *AdminStatisticsService) GetTopApplications(limit int) ([]*dto.TopApplicationResponse, error) {
	if limit <= 0 {
		limit = 10
	}
	db := s.db

	var results []struct {
		AppID        string
		PushCount    int64
		SuccessCount int64
	}

	// 聚合查询
	err := s.userTaskQuery().
		Select("app_id, COUNT(*) as push_count, SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count").
		Group("app_id").
		Order("push_count DESC").
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	// 获取应用详情
	items := make([]*dto.TopApplicationResponse, 0, len(results))
	for _, res := range results {
		var app model.Application
		// AppID 是 string
		appName := "未知应用"
		var id uint
		err := db.Unscoped().Where("app_id = ?", res.AppID).First(&app).Error
		if err == nil {
			appName = app.AppName
			id = app.ID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("failed to load top application %q: %w", res.AppID, err)
		}

		successRate := "0.00%"
		if res.PushCount > 0 {
			successRate = fmt.Sprintf("%.2f%%", float64(res.SuccessCount)/float64(res.PushCount)*100)
		}

		items = append(items, &dto.TopApplicationResponse{
			ID:           id,
			AppID:        res.AppID,
			AppName:      appName,
			PushCount:    res.PushCount,
			SuccessCount: res.SuccessCount,
			SuccessRate:  successRate,
		})
	}

	return items, nil
}

// GetRecentActivities 获取近期活动
func (s *AdminStatisticsService) GetRecentActivities(limit int) ([]*dto.RecentActivityResponse, error) {
	if limit <= 0 {
		limit = 10
	}
	db := s.db

	var tasks []*model.PushTask
	if err := s.userTaskQuery().Order("created_at DESC").Limit(limit).Find(&tasks).Error; err != nil {
		return nil, err
	}

	// 缓存 App Name
	appMap := make(map[string]string)

	items := make([]*dto.RecentActivityResponse, 0, len(tasks))
	for _, task := range tasks {
		appName, ok := appMap[task.AppID]
		if !ok {
			var app model.Application
			err := db.Unscoped().Where("app_id = ?", task.AppID).First(&app).Error
			if err == nil {
				appName = app.AppName
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				appName = "未知应用"
			} else {
				return nil, fmt.Errorf("failed to load application %q for recent activity: %w", task.AppID, err)
			}
			appMap[task.AppID] = appName
		}

		desc := fmt.Sprintf("推送消息 (TaskID: %s) 状态: %s", task.TaskID, task.Status)

		items = append(items, &dto.RecentActivityResponse{
			ID:          task.ID,
			Description: desc,
			AppName:     appName,
			CreatedAt:   timeutil.FormatRFC3339(task.CreatedAt),
		})
	}

	return items, nil
}
