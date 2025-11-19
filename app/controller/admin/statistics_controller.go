package admin

import (
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
	adminService := service.NewAdminService()
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
