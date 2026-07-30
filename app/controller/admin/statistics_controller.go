package admin

import (
	"errors"
	"strconv"

	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
)

type adminStatisticsService interface {
	GetStatistics(req *dto.StatisticsRequest) (*dto.StatisticsResponse, error)
	GetDashboard() (*dto.DashboardResponse, error)
	GetTopApplications(limit int) ([]*dto.TopApplicationResponse, error)
	GetRecentActivities(limit int) ([]*dto.RecentActivityResponse, error)
}

// StatisticsController 统计分析控制器
type StatisticsController struct {
	newService func() adminStatisticsService
}

func (c StatisticsController) statisticsService() adminStatisticsService {
	if c.newService != nil {
		return c.newService()
	}
	return service.NewAdminStatisticsService()
}

// GetStatistics 获取统计数据
func (c StatisticsController) GetStatistics(ctx httpInterfaces.RouterContextInterface) {
	adminService := c.statisticsService()
	var req dto.StatisticsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.GetStatistics(&req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidStatisticsRequest) {
			controller.ErrorResponse(ctx, 400, err.Error())
			return
		}
		controller.ErrorResponse(ctx, 500, "failed to get statistics: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetDashboard 获取仪表盘数据
func (c StatisticsController) GetDashboard(ctx httpInterfaces.RouterContextInterface) {
	adminService := c.statisticsService()
	resp, err := adminService.GetDashboard()
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get dashboard data: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetTopApplications 获取热门应用
func (c StatisticsController) GetTopApplications(ctx httpInterfaces.RouterContextInterface) {
	adminService := c.statisticsService()
	limitStr := ctx.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)

	resp, err := adminService.GetTopApplications(limit)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get top applications: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetRecentActivities 获取近期活动
func (c StatisticsController) GetRecentActivities(ctx httpInterfaces.RouterContextInterface) {
	adminService := c.statisticsService()
	limitStr := ctx.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)

	resp, err := adminService.GetRecentActivities(limit)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get recent activities: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}
