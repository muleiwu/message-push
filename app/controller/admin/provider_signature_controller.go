package admin

import (
	"strconv"

	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
)

// ProviderSignatureController 供应商签名管理控制器
type ProviderSignatureController struct {
}

// GetGlobalSignatureList 获取全局签名分页列表
func (c ProviderSignatureController) GetGlobalSignatureList(ctx httpInterfaces.RouterContextInterface) {
	signatureService := service.NewAdminProviderSignatureService()

	var req dto.ProviderSignatureListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := signatureService.GetGlobalSignatureList(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get signature list: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetSignatureList 获取签名列表
func (c ProviderSignatureController) GetSignatureList(ctx httpInterfaces.RouterContextInterface) {
	signatureService := service.NewAdminProviderSignatureService()

	accountIDStr := ctx.Param("id")
	accountID, err := strconv.ParseUint(accountIDStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid account id")
		return
	}

	var req dto.ProviderSignatureListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := signatureService.GetSignatureList(uint(accountID), req.Status)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get signature list: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// CreateSignature 创建签名
func (c ProviderSignatureController) CreateSignature(ctx httpInterfaces.RouterContextInterface) {
	signatureService := service.NewAdminProviderSignatureService()

	accountIDStr := ctx.Param("id")
	accountID, err := strconv.ParseUint(accountIDStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid account id")
		return
	}

	var req dto.CreateProviderSignatureRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := signatureService.CreateSignature(uint(accountID), &req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to create signature: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// UpdateSignature 更新签名
func (c ProviderSignatureController) UpdateSignature(ctx httpInterfaces.RouterContextInterface) {
	signatureService := service.NewAdminProviderSignatureService()

	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.UpdateProviderSignatureRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	if err := signatureService.UpdateSignature(uint(id), &req); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to update signature: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, map[string]any{"message": "updated successfully"})
}

// DeleteSignature 删除签名
func (c ProviderSignatureController) DeleteSignature(ctx httpInterfaces.RouterContextInterface) {
	signatureService := service.NewAdminProviderSignatureService()

	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	if err := signatureService.DeleteSignature(uint(id)); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to delete signature: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, map[string]any{"message": "deleted successfully"})
}

// GetSignature 获取签名详情
func (c ProviderSignatureController) GetSignature(ctx httpInterfaces.RouterContextInterface) {
	signatureService := service.NewAdminProviderSignatureService()

	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	resp, err := signatureService.GetSignatureByID(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 404, "signature not found")
		return
	}

	controller.SuccessResponse(ctx, resp)
}
