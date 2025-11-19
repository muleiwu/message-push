package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// StatisticsController 统计分析控制器
type StatisticsController struct {
}

// GetStatistics 获取统计数据
func (c StatisticsController) GetStatistics(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminStatisticsService()
	var req dto.StatisticsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.GetStatistics(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get statistics: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetDashboard 获取仪表盘数据
func (c StatisticsController) GetDashboard(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminStatisticsService()
	resp, err := adminService.GetDashboard()
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get dashboard data: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetTopApplications 获取热门应用
func (c StatisticsController) GetTopApplications(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminStatisticsService()
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
func (c StatisticsController) GetRecentActivities(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminStatisticsService()
	limitStr := ctx.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)

	resp, err := adminService.GetRecentActivities(limit)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get recent activities: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}
