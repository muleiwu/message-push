package service

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
)

// AdminStatisticsService 统计分析服务
type AdminStatisticsService struct {
	db *gorm.DB
}

func (s *AdminStatisticsService) userTaskQuery() *gorm.DB {
	return s.db.Model(&model.PushTask{}).Where("app_id <> ?", adminTestAppID)
}

// NewAdminStatisticsService 创建统计分析服务实例
func NewAdminStatisticsService() *AdminStatisticsService {
	return &AdminStatisticsService{db: helper.GetDatabase()}
}

// GetStatistics 获取推送统计
func (s *AdminStatisticsService) GetStatistics(req *dto.StatisticsRequest) (*dto.StatisticsResponse, error) {
	db := s.db

	// 基本查询
	query := s.userTaskQuery().Where("DATE(created_at) >= ? AND DATE(created_at) <= ?", req.StartDate, req.EndDate)

	// 条件过滤
	if req.AppID > 0 {
		// PushTask 中的 AppID 是 string 类型 (app_id string)，这里 req.AppID 是 uint
		// 需要根据 uint ID 查出 string AppID
		var app model.Application
		if err := db.First(&app, req.AppID).Error; err == nil {
			query = query.Where("app_id = ?", app.AppID)
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// 不存在的应用筛选应返回空集，不能退化成“所有应用”。
			query = query.Where("1 = 0")
		} else {
			return nil, fmt.Errorf("failed to resolve application filter: %w", err)
		}
	}
	if req.ChannelID > 0 {
		query = query.Where("channel_id = ?", req.ChannelID)
	}

	// 汇总统计
	var summary struct {
		Total   int64
		Success int64
		Failed  int64
	}

	// 使用 clone 避免互相影响
	qTotal := query.Session(&gorm.Session{})
	qSuccess := query.Session(&gorm.Session{})
	qFailed := query.Session(&gorm.Session{})

	qTotal.Count(&summary.Total)
	qSuccess.Where("status = ?", "success").Count(&summary.Success)
	qFailed.Where("status = ?", "failed").Count(&summary.Failed)

	// 每日统计
	var dailyStats []struct {
		Date    string
		Total   int64
		Success int64
		Failed  int64
	}

	// 复用同一个筛选查询，确保汇总和每日趋势的应用、通道口径一致。
	query.Session(&gorm.Session{}).
		Select("DATE(created_at) as date, COUNT(*) as total, SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success, SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed").
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&dailyStats)

	// 构建响应
	response := &dto.StatisticsResponse{}
	response.Summary.TotalCount = summary.Total
	response.Summary.SuccessCount = summary.Success
	response.Summary.FailureCount = summary.Failed
	if summary.Total > 0 {
		response.Summary.SuccessRate = fmt.Sprintf("%.2f%%", float64(summary.Success)/float64(summary.Total)*100)
	} else {
		response.Summary.SuccessRate = "0.00%"
	}

	response.Daily = make([]*dto.DailyStatistics, 0, len(dailyStats))
	for _, stat := range dailyStats {
		successRate := "0.00%"
		if stat.Total > 0 {
			successRate = fmt.Sprintf("%.2f%%", float64(stat.Success)/float64(stat.Total)*100)
		}
		response.Daily = append(response.Daily, &dto.DailyStatistics{
			Date:         stat.Date,
			TotalCount:   stat.Total,
			SuccessCount: stat.Success,
			FailureCount: stat.Failed,
			SuccessRate:  successRate,
		})
	}

	return response, nil
}

// GetDashboard 获取仪表盘数据
func (s *AdminStatisticsService) GetDashboard() (*dto.DashboardResponse, error) {
	db := s.db
	resp := &dto.DashboardResponse{}

	// 1. 统计 Applications
	db.Model(&model.Application{}).Count(&resp.TotalApplications)
	db.Model(&model.Application{}).Where("status = ?", 1).Count(&resp.ActiveApplications)

	// 2. 统计 Channels
	db.Model(&model.Channel{}).Count(&resp.TotalChannels)
	db.Model(&model.Channel{}).Where("status = ?", 1).Count(&resp.ActiveChannels)

	// 3. 统计 Provider Accounts
	db.Model(&model.ProviderAccount{}).Count(&resp.TotalProviders)
	db.Model(&model.ProviderAccount{}).Where("status = ?", 1).Count(&resp.ActiveProviders)

	// 4. 统计今日推送
	todayStart := time.Now().Format("2006-01-02 00:00:00")
	todayEnd := time.Now().Format("2006-01-02 23:59:59")

	var todayStats struct {
		Total   int64
		Success int64
		Failed  int64
	}
	s.userTaskQuery().
		Select("COUNT(*) as total, SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success, SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed").
		Where("created_at >= ? AND created_at <= ?", todayStart, todayEnd).
		Scan(&todayStats)

	resp.TodayPushCount = todayStats.Total
	resp.TodaySuccessCount = todayStats.Success
	resp.TodayFailedCount = todayStats.Failed
	if resp.TodayPushCount > 0 {
		resp.TodaySuccessRate = fmt.Sprintf("%.2f%%", float64(resp.TodaySuccessCount)/float64(resp.TodayPushCount)*100)
	} else {
		resp.TodaySuccessRate = "0.00%"
	}

	// 5. 统计总推送量
	s.userTaskQuery().Count(&resp.TotalPushCount)

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
		if err := db.Where("app_id = ?", res.AppID).First(&app).Error; err == nil {
			appName = app.AppName
			id = app.ID
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
			if err := db.Where("app_id = ?", task.AppID).First(&app).Error; err == nil {
				appName = app.AppName
			} else {
				appName = "未知应用"
			}
			appMap[task.AppID] = appName
		}

		desc := fmt.Sprintf("推送消息 (TaskID: %s) 状态: %s", task.TaskID, task.Status)

		items = append(items, &dto.RecentActivityResponse{
			ID:          task.ID,
			Description: desc,
			AppName:     appName,
			CreatedAt:   task.CreatedAt.Format(time.RFC3339),
		})
	}

	return items, nil
}
