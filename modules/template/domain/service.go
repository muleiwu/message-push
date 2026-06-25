package domain

import "cnb.cool/mliev/push/message-push/app/dto"

// Service 模板管理服务端口：系统模板与供应商模板的增删改查。
// 基础设施层提供实现，经 assembly 注册到 go-web 容器，供管理端控制器调用。
type Service interface {
	// 系统模板
	CreateMessageTemplate(req *dto.CreateMessageTemplateRequest) (*dto.MessageTemplateResponse, error)
	UpdateMessageTemplate(id uint, req *dto.UpdateMessageTemplateRequest) (*dto.MessageTemplateResponse, error)
	GetMessageTemplate(id uint) (*dto.MessageTemplateResponse, error)
	DeleteMessageTemplate(id uint) error
	ListMessageTemplates(req *dto.MessageTemplateListRequest) (*dto.MessageTemplateListResponse, error)

	// 供应商模板
	CreateProviderTemplate(req *dto.CreateProviderTemplateRequest) (*dto.ProviderTemplateResponse, error)
	UpdateProviderTemplate(id uint, req *dto.UpdateProviderTemplateRequest) (*dto.ProviderTemplateResponse, error)
	GetProviderTemplate(id uint) (*dto.ProviderTemplateResponse, error)
	DeleteProviderTemplate(id uint) error
	ListProviderTemplates(req *dto.ProviderTemplateListRequest) (*dto.ProviderTemplateListResponse, error)
}
