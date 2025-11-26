package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// TaskController 任务管理控制器
type TaskController struct {
}

// GetPushTaskList 获取推送任务列表
func (c TaskController) GetPushTaskList(ctx *gin.Context, helper interfaces.HelperInterface) {
	taskService := service.NewAdminTaskService()
	var req dto.PushTaskListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := taskService.GetPushTaskList(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get push tasks: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetPushTask 获取单个推送任务详情
func (c TaskController) GetPushTask(ctx *gin.Context, helper interfaces.HelperInterface) {
	taskService := service.NewAdminTaskService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid task id")
		return
	}

	resp, err := taskService.GetPushTask(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get push task: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetPushBatchTaskList 获取批量任务列表
func (c TaskController) GetPushBatchTaskList(ctx *gin.Context, helper interfaces.HelperInterface) {
	taskService := service.NewAdminTaskService()
	var req dto.PushBatchTaskListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := taskService.GetPushBatchTaskList(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get batch tasks: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetPushBatchTask 获取单个批量任务详情
func (c TaskController) GetPushBatchTask(ctx *gin.Context, helper interfaces.HelperInterface) {
	taskService := service.NewAdminTaskService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid batch task id")
		return
	}

	resp, err := taskService.GetPushBatchTask(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get batch task: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetBatchTaskDetails 获取批次下的所有任务
func (c TaskController) GetBatchTaskDetails(ctx *gin.Context, helper interfaces.HelperInterface) {
	taskService := service.NewAdminTaskService()
	batchID := ctx.Param("id")

	// 获取分页参数
	pageStr := ctx.DefaultQuery("page", "1")
	pageSizeStr := ctx.DefaultQuery("page_size", "20")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	resp, err := taskService.GetTasksByBatchID(batchID, page, pageSize)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get batch task details: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}
