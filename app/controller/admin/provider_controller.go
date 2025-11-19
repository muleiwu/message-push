package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// ProviderController 服务商管理控制器
type ProviderController struct {
}

// CreateProvider 创建服务商
func (c ProviderController) CreateProvider(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminService()
	var req dto.CreateProviderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.CreateProvider(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to create provider: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetProviderList 获取服务商列表
func (c ProviderController) GetProviderList(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminService()
	var req dto.ProviderListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.GetProviderList(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get provider list: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetProvider 获取服务商详情
func (c ProviderController) GetProvider(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	resp, err := adminService.GetProviderByID(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 404, "provider not found")
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// UpdateProvider 更新服务商
func (c ProviderController) UpdateProvider(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.UpdateProviderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	if err := adminService.UpdateProvider(uint(id), &req); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to update provider: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, gin.H{"message": "updated successfully"})
}

// DeleteProvider 删除服务商
func (c ProviderController) DeleteProvider(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	if err := adminService.DeleteProvider(uint(id)); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to delete provider: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, gin.H{"message": "deleted successfully"})
}
