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

// GetLogsByTaskID 根据任务ID获取推送日志
func (c LogController) GetLogsByTaskID(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminLogService()
	taskID := ctx.Param("task_id")
	if taskID == "" {
		controller.ErrorResponse(ctx, 400, "task_id is required")
		return
	}

	resp, err := adminService.GetLogsByTaskID(taskID)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get logs: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetCallbackLogsByTaskID 根据任务ID获取回调日志
func (c LogController) GetCallbackLogsByTaskID(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminLogService()
	taskID := ctx.Param("task_id")
	if taskID == "" {
		controller.ErrorResponse(ctx, 400, "task_id is required")
		return
	}

	resp, err := adminService.GetCallbackLogsByTaskID(taskID)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get callback logs: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetWebhookLogsByTaskID 根据任务ID获取Webhook日志
func (c LogController) GetWebhookLogsByTaskID(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminLogService()
	taskID := ctx.Param("task_id")
	if taskID == "" {
		controller.ErrorResponse(ctx, 400, "task_id is required")
		return
	}

	resp, err := adminService.GetWebhookLogsByTaskID(taskID)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get webhook logs: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}
