package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// LogController 日志管理控制器
type LogController struct {
}

// GetLogList 获取日志列表
func (c LogController) GetLogList(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminLogService()
	var req dto.LogListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.GetLogList(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get logs: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetLog 获取日志详情
func (c LogController) GetLog(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminLogService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid log id")
		return
	}

	resp, err := adminService.GetLog(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get log: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}
