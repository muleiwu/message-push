package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// ApplicationController 应用管理控制器
type ApplicationController struct {
}

// CreateApplication 创建应用
func (c ApplicationController) CreateApplication(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminApplicationService()
	var req dto.CreateApplicationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.CreateApplication(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to create application: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetApplicationList 获取应用列表
func (c ApplicationController) GetApplicationList(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminApplicationService()
	var req dto.ApplicationListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.GetApplicationList(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get application list: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetApplication 获取应用详情
func (c ApplicationController) GetApplication(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminApplicationService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	resp, err := adminService.GetApplicationByID(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 404, "application not found")
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// UpdateApplication 更新应用
func (c ApplicationController) UpdateApplication(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminApplicationService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.UpdateApplicationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	if err := adminService.UpdateApplication(uint(id), &req); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to update application: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, gin.H{"message": "updated successfully"})
}

// DeleteApplication 删除应用
func (c ApplicationController) DeleteApplication(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminApplicationService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	if err := adminService.DeleteApplication(uint(id)); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to delete application: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, gin.H{"message": "deleted successfully"})
}

// RegenerateSecret 重新生成密钥
func (c ApplicationController) RegenerateSecret(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminApplicationService()
	var req dto.RegenerateSecretRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.RegenerateSecret(req.AppID)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to regenerate secret: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}
