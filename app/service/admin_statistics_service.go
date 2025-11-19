package service

import (
	"fmt"

	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// AdminStatisticsService 统计分析服务
type AdminStatisticsService struct{}

// NewAdminStatisticsService 创建统计分析服务实例
func NewAdminStatisticsService() *AdminStatisticsService {
	return &AdminStatisticsService{}
}

// GetStatistics 获取推送统计
func (s *AdminStatisticsService) GetStatistics(req *dto.StatisticsRequest) (*dto.StatisticsResponse, error) {
	db := helper.GetHelper().GetDatabase()

	// 基本查询
	query := db.Model(&model.PushLog{}).Where("DATE(created_at) >= ? AND DATE(created_at) <= ?", req.StartDate, req.EndDate)

	// 条件过滤
	if req.AppID > 0 {
		query = query.Where("app_id = ?", req.AppID)
	}
	if req.ChannelID > 0 {
		query = query.Where("provider_channel_id = ?", req.ChannelID)
	}

	// 汇总统计
	var summary struct {
		Total   int64
		Success int64
		Failed  int64
	}

	query.Count(&summary.Total)
	query.Where("status = ?", "success").Count(&summary.Success)
	summary.Failed = summary.Total - summary.Success

	// 每日统计
	var dailyStats []struct {
		Date    string
		Total   int64
		Success int64
	}

	db.Model(&model.PushLog{}).
		Select("DATE(created_at) as date, COUNT(*) as total, SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success").
		Where("DATE(created_at) >= ? AND DATE(created_at) <= ?", req.StartDate, req.EndDate).
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
		failed := stat.Total - stat.Success
		successRate := "0.00%"
		if stat.Total > 0 {
			successRate = fmt.Sprintf("%.2f%%", float64(stat.Success)/float64(stat.Total)*100)
		}
		response.Daily = append(response.Daily, &dto.DailyStatistics{
			Date:         stat.Date,
			TotalCount:   stat.Total,
			SuccessCount: stat.Success,
			FailureCount: failed,
			SuccessRate:  successRate,
		})
	}

	return response, nil
}
