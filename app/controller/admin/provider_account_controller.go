package admin

import (
	"strconv"

	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
)

// ProviderAccountController 服务商账号配置管理控制器
type ProviderAccountController struct {
}

// GetAvailableProviders 获取可用的服务商列表
func (c ProviderAccountController) GetAvailableProviders(ctx httpInterfaces.RouterContextInterface) {
	adminService := service.NewAdminProviderAccountService()
	providerType := ctx.Query("provider_type")

	resp, err := adminService.GetAvailableProviders(providerType)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get available providers: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetProviderConfigFields 获取服务商配置字段定义
func (c ProviderAccountController) GetProviderConfigFields(ctx httpInterfaces.RouterContextInterface) {
	adminService := service.NewAdminProviderAccountService()
	providerCode := ctx.Param("providerCode")

	resp, err := adminService.GetProviderConfigFields(providerCode)
	if err != nil {
		controller.ErrorResponse(ctx, 404, "provider not found: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// CreateProviderAccount 创建服务商账号配置
func (c ProviderAccountController) CreateProviderAccount(ctx httpInterfaces.RouterContextInterface) {
	adminService := service.NewAdminProviderAccountService()
	var req dto.CreateProviderAccountRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.CreateProviderAccount(ctx, &req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to create provider account: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetProviderAccountList 获取服务商账号列表
func (c ProviderAccountController) GetProviderAccountList(ctx httpInterfaces.RouterContextInterface) {
	adminService := service.NewAdminProviderAccountService()
	var req dto.ProviderAccountListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.GetProviderAccountList(ctx, &req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get provider account list: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetProviderAccount 获取服务商账号详情
func (c ProviderAccountController) GetProviderAccount(ctx httpInterfaces.RouterContextInterface) {
	adminService := service.NewAdminProviderAccountService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	resp, err := adminService.GetProviderAccountByID(ctx, uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 404, "provider account not found")
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// UpdateProviderAccount 更新服务商账号
func (c ProviderAccountController) UpdateProviderAccount(ctx httpInterfaces.RouterContextInterface) {
	adminService := service.NewAdminProviderAccountService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.UpdateProviderAccountRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	if err := adminService.UpdateProviderAccount(uint(id), &req); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to update provider account: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, map[string]any{"message": "updated successfully"})
}

// DeleteProviderAccount 删除服务商账号
func (c ProviderAccountController) DeleteProviderAccount(ctx httpInterfaces.RouterContextInterface) {
	adminService := service.NewAdminProviderAccountService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	if err := adminService.DeleteProviderAccount(uint(id)); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to delete provider account: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, map[string]any{"message": "deleted successfully"})
}

// GetActiveProviderAccounts 获取活跃服务商账号列表
func (c ProviderAccountController) GetActiveProviderAccounts(ctx httpInterfaces.RouterContextInterface) {
	adminService := service.NewAdminProviderAccountService()
	providerType := ctx.Query("provider_type")

	resp, err := adminService.GetActiveProviderAccounts(providerType)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get active provider accounts: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// TestProviderAccount 测试服务商账号配置
func (c ProviderAccountController) TestProviderAccount(ctx httpInterfaces.RouterContextInterface) {
	adminService := service.NewAdminProviderAccountService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.TestProviderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.TestProviderAccount(uint(id), &req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "test failed: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}
