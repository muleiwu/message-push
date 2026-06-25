package admin

import (
	"strconv"

	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/modules/template"
)

// TemplateController 模板管理控制器
type TemplateController struct {
}

// ========== 系统模板管理 ==========

// CreateMessageTemplate 创建系统模板
func (c TemplateController) CreateMessageTemplate(ctx httpInterfaces.RouterContextInterface) {
	templateService := template.GetService()
	var req dto.CreateMessageTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := templateService.CreateMessageTemplate(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to create template: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// UpdateMessageTemplate 更新系统模板
func (c TemplateController) UpdateMessageTemplate(ctx httpInterfaces.RouterContextInterface) {
	templateService := template.GetService()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.UpdateMessageTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := templateService.UpdateMessageTemplate(uint(id), &req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to update template: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetMessageTemplate 获取系统模板详情
func (c TemplateController) GetMessageTemplate(ctx httpInterfaces.RouterContextInterface) {
	templateService := template.GetService()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	resp, err := templateService.GetMessageTemplate(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get template: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// DeleteMessageTemplate 删除系统模板
func (c TemplateController) DeleteMessageTemplate(ctx httpInterfaces.RouterContextInterface) {
	templateService := template.GetService()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	if err := templateService.DeleteMessageTemplate(uint(id)); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to delete template: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, nil)
}

// ListMessageTemplates 查询系统模板列表
func (c TemplateController) ListMessageTemplates(ctx httpInterfaces.RouterContextInterface) {
	templateService := template.GetService()
	var req dto.MessageTemplateListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	// 设置默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 10
	}

	resp, err := templateService.ListMessageTemplates(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to list templates: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// ========== 供应商模板管理 ==========

// CreateProviderTemplate 创建供应商模板
func (c TemplateController) CreateProviderTemplate(ctx httpInterfaces.RouterContextInterface) {
	templateService := template.GetService()
	var req dto.CreateProviderTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := templateService.CreateProviderTemplate(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to create provider template: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// UpdateProviderTemplate 更新供应商模板
func (c TemplateController) UpdateProviderTemplate(ctx httpInterfaces.RouterContextInterface) {
	templateService := template.GetService()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.UpdateProviderTemplateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := templateService.UpdateProviderTemplate(uint(id), &req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to update provider template: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetProviderTemplate 获取供应商模板详情
func (c TemplateController) GetProviderTemplate(ctx httpInterfaces.RouterContextInterface) {
	templateService := template.GetService()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	resp, err := templateService.GetProviderTemplate(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get provider template: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// DeleteProviderTemplate 删除供应商模板
func (c TemplateController) DeleteProviderTemplate(ctx httpInterfaces.RouterContextInterface) {
	templateService := template.GetService()
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	if err := templateService.DeleteProviderTemplate(uint(id)); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to delete provider template: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, nil)
}

// ListProviderTemplates 查询供应商模板列表
func (c TemplateController) ListProviderTemplates(ctx httpInterfaces.RouterContextInterface) {
	templateService := template.GetService()
	var req dto.ProviderTemplateListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	// 设置默认值
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 10
	}

	resp, err := templateService.ListProviderTemplates(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to list provider templates: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}
