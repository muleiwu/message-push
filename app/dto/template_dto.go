package dto

import "time"

// ========== 系统模板 DTO ==========

// CreateMessageTemplateRequest 创建系统模板请求
type CreateMessageTemplateRequest struct {
	TemplateCode string   `json:"template_code" binding:"required"`
	TemplateName string   `json:"template_name" binding:"required"`
	MessageType  string   `json:"message_type" binding:"required"`
	Content      string   `json:"content" binding:"required"`
	Variables    []string `json:"variables"`
	Description  string   `json:"description"`
	Status       *int8    `json:"status"`
}

// UpdateMessageTemplateRequest 更新系统模板请求
type UpdateMessageTemplateRequest struct {
	TemplateName string   `json:"template_name"`
	MessageType  string   `json:"message_type"`
	Content      string   `json:"content"`
	Variables    []string `json:"variables"`
	Description  string   `json:"description"`
	Status       *int8    `json:"status"`
}

// MessageTemplateResponse 系统模板响应
type MessageTemplateResponse struct {
	ID           uint      `json:"id"`
	TemplateCode string    `json:"template_code"`
	TemplateName string    `json:"template_name"`
	MessageType  string    `json:"message_type"`
	Content      string    `json:"content"`
	Variables    []string  `json:"variables"`
	Description  string    `json:"description"`
	Status       int8      `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// MessageTemplateListRequest 系统模板列表查询请求
type MessageTemplateListRequest struct {
	MessageType string `form:"message_type"`
	Status      *int8  `form:"status"`
	Page        int    `form:"page" binding:"required,min=1"`
	PageSize    int    `form:"page_size" binding:"required,min=1,max=100"`
}

// MessageTemplateListResponse 系统模板列表响应
type MessageTemplateListResponse struct {
	Items []*MessageTemplateResponse `json:"items"`
	Total int64                      `json:"total"`
	Page  int                        `json:"page"`
	Size  int                        `json:"size"`
}

// ========== 供应商模板 DTO ==========

// CreateProviderTemplateRequest 创建供应商模板请求
type CreateProviderTemplateRequest struct {
	ProviderID      uint     `json:"provider_id" binding:"required"`
	TemplateCode    string   `json:"template_code" binding:"required"`
	TemplateName    string   `json:"template_name" binding:"required"`
	TemplateContent string   `json:"template_content"`
	Variables       []string `json:"variables"`
	Status          *int8    `json:"status"`
	Remark          string   `json:"remark"`
}

// UpdateProviderTemplateRequest 更新供应商模板请求
type UpdateProviderTemplateRequest struct {
	TemplateName    string   `json:"template_name"`
	TemplateContent string   `json:"template_content"`
	Variables       []string `json:"variables"`
	Status          *int8    `json:"status"`
	Remark          string   `json:"remark"`
}

// ProviderTemplateResponse 供应商模板响应
type ProviderTemplateResponse struct {
	ID              uint                    `json:"id"`
	ProviderID      uint                    `json:"provider_id"`
	TemplateCode    string                  `json:"template_code"`
	TemplateName    string                  `json:"template_name"`
	TemplateContent string                  `json:"template_content"`
	Variables       []string                `json:"variables"`
	Status          int8                    `json:"status"`
	Remark          string                  `json:"remark"`
	ProviderAccount *SimpleProviderResponse `json:"provider_account,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

// SimpleProviderResponse 简单供应商信息
type SimpleProviderResponse struct {
	ID           uint   `json:"id"`
	AccountCode  string `json:"account_code"`
	AccountName  string `json:"account_name"`
	ProviderCode string `json:"provider_code"`
	ProviderType string `json:"provider_type"`
}

// ProviderTemplateListRequest 供应商模板列表查询请求
type ProviderTemplateListRequest struct {
	ProviderID *uint `form:"provider_id"`
	Status     *int8 `form:"status"`
	Page       int   `form:"page" binding:"required,min=1"`
	PageSize   int   `form:"page_size" binding:"required,min=1,max=100"`
}

// ProviderTemplateListResponse 供应商模板列表响应
type ProviderTemplateListResponse struct {
	Items []*ProviderTemplateResponse `json:"items"`
	Total int64                       `json:"total"`
	Page  int                         `json:"page"`
	Size  int                         `json:"size"`
}

// ========== 模板绑定 DTO ==========

// CreateTemplateBindingRequest 创建模板绑定请求
type CreateTemplateBindingRequest struct {
	MessageTemplateID  uint              `json:"message_template_id" binding:"required"`
	ProviderTemplateID uint              `json:"provider_template_id" binding:"required"`
	ProviderID         uint              `json:"provider_id" binding:"required"`
	ParamMapping       map[string]string `json:"param_mapping"`
	Status             *int8             `json:"status"`
	Priority           *int              `json:"priority"`
}

// UpdateTemplateBindingRequest 更新模板绑定请求
type UpdateTemplateBindingRequest struct {
	ParamMapping map[string]string `json:"param_mapping"`
	Status       *int8             `json:"status"`
	Priority     *int              `json:"priority"`
}

// TemplateBindingResponse 模板绑定响应
type TemplateBindingResponse struct {
	ID                 uint                        `json:"id"`
	MessageTemplateID  uint                        `json:"message_template_id"`
	ProviderTemplateID uint                        `json:"provider_template_id"`
	ProviderID         uint                        `json:"provider_id"`
	ParamMapping       map[string]string           `json:"param_mapping"`
	Status             int8                        `json:"status"`
	Priority           int                         `json:"priority"`
	MessageTemplate    *SimpleMessageTemplateInfo  `json:"message_template,omitempty"`
	ProviderTemplate   *SimpleProviderTemplateInfo `json:"provider_template,omitempty"`
	ProviderAccount    *SimpleProviderResponse     `json:"provider_account,omitempty"`
	CreatedAt          time.Time                   `json:"created_at"`
	UpdatedAt          time.Time                   `json:"updated_at"`
}

// SimpleMessageTemplateInfo 简单系统模板信息
type SimpleMessageTemplateInfo struct {
	ID           uint   `json:"id"`
	TemplateCode string `json:"template_code"`
	TemplateName string `json:"template_name"`
	MessageType  string `json:"message_type"`
}

// SimpleProviderTemplateInfo 简单供应商模板信息
type SimpleProviderTemplateInfo struct {
	ID           uint   `json:"id"`
	TemplateCode string `json:"template_code"`
	TemplateName string `json:"template_name"`
}

// TemplateBindingListRequest 模板绑定列表查询请求
type TemplateBindingListRequest struct {
	MessageTemplateID *uint `form:"message_template_id"`
	ProviderID        *uint `form:"provider_id"`
	Status            *int8 `form:"status"`
	Page              int   `form:"page" binding:"required,min=1"`
	PageSize          int   `form:"page_size" binding:"required,min=1,max=100"`
}

// TemplateBindingListResponse 模板绑定列表响应
type TemplateBindingListResponse struct {
	Items []*TemplateBindingResponse `json:"items"`
	Total int64                      `json:"total"`
	Page  int                        `json:"page"`
	Size  int                        `json:"size"`
}
