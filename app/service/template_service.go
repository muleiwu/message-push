package service

import (
	"errors"
	"fmt"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"gorm.io/gorm"
)

// TemplateService 模板管理服务
type TemplateService struct {
	messageTemplateDAO  *dao.MessageTemplateDAO
	providerTemplateDAO *dao.ProviderTemplateDAO
	providerAccountDAO  *dao.ProviderAccountDAO
}

// NewTemplateService 创建模板管理服务
func NewTemplateService() *TemplateService {
	return &TemplateService{
		messageTemplateDAO:  dao.NewMessageTemplateDAO(),
		providerTemplateDAO: dao.NewProviderTemplateDAO(),
		providerAccountDAO:  dao.NewProviderAccountDAO(),
	}
}

// ========== 系统模板管理 ==========

// CreateMessageTemplate 创建系统模板
func (s *TemplateService) CreateMessageTemplate(req *dto.CreateMessageTemplateRequest) (*dto.MessageTemplateResponse, error) {
	// 检查模板代码是否已存在
	exists, err := s.messageTemplateDAO.ExistsByCode(req.TemplateCode, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to check template code: %w", err)
	}
	if exists {
		return nil, errors.New("template code already exists")
	}

	// 创建模板
	template := &model.MessageTemplate{
		TemplateCode: req.TemplateCode,
		TemplateName: req.TemplateName,
		MessageType:  req.MessageType,
		Content:      req.Content,
		Description:  req.Description,
		Status:       1,
	}

	if req.Status != nil {
		template.Status = *req.Status
	}

	if req.Variables != nil {
		if err := template.SetVariables(req.Variables); err != nil {
			return nil, fmt.Errorf("failed to set variables: %w", err)
		}
	}

	if err := s.messageTemplateDAO.Create(template); err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	return s.buildMessageTemplateResponse(template)
}

// UpdateMessageTemplate 更新系统模板
func (s *TemplateService) UpdateMessageTemplate(id uint, req *dto.UpdateMessageTemplateRequest) (*dto.MessageTemplateResponse, error) {
	// 获取现有模板
	template, err := s.messageTemplateDAO.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("template not found")
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// 更新字段
	if req.TemplateName != "" {
		template.TemplateName = req.TemplateName
	}
	if req.MessageType != "" {
		template.MessageType = req.MessageType
	}
	if req.Content != "" {
		template.Content = req.Content
	}
	if req.Description != "" {
		template.Description = req.Description
	}
	if req.Status != nil {
		template.Status = *req.Status
	}
	if req.Variables != nil {
		if err := template.SetVariables(req.Variables); err != nil {
			return nil, fmt.Errorf("failed to set variables: %w", err)
		}
	}

	if err := s.messageTemplateDAO.Update(template); err != nil {
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	return s.buildMessageTemplateResponse(template)
}

// GetMessageTemplate 获取系统模板
func (s *TemplateService) GetMessageTemplate(id uint) (*dto.MessageTemplateResponse, error) {
	template, err := s.messageTemplateDAO.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("template not found")
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	return s.buildMessageTemplateResponse(template)
}

// DeleteMessageTemplate 删除系统模板
func (s *TemplateService) DeleteMessageTemplate(id uint) error {
	return s.messageTemplateDAO.Delete(id)
}

// ListMessageTemplates 查询系统模板列表
func (s *TemplateService) ListMessageTemplates(req *dto.MessageTemplateListRequest) (*dto.MessageTemplateListResponse, error) {
	templates, total, err := s.messageTemplateDAO.List(req.MessageType, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	list := make([]*dto.MessageTemplateResponse, 0, len(templates))
	for _, template := range templates {
		resp, err := s.buildMessageTemplateResponse(template)
		if err != nil {
			continue
		}
		list = append(list, resp)
	}

	return &dto.MessageTemplateListResponse{
		Items: list,
		Total: total,
		Page:  req.Page,
		Size:  req.PageSize,
	}, nil
}

// ========== 供应商模板管理 ==========

// CreateProviderTemplate 创建供应商模板
func (s *TemplateService) CreateProviderTemplate(req *dto.CreateProviderTemplateRequest) (*dto.ProviderTemplateResponse, error) {
	// 检查供应商是否存在
	_, err := s.providerAccountDAO.GetByID(req.ProviderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("provider not found")
		}
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	// 检查模板代码是否已存在
	exists, err := s.providerTemplateDAO.ExistsByProviderAndCode(req.ProviderID, req.TemplateCode, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to check template code: %w", err)
	}
	if exists {
		return nil, errors.New("template code already exists for this provider")
	}

	// 创建模板
	template := &model.ProviderTemplate{
		ProviderID:      req.ProviderID,
		TemplateCode:    req.TemplateCode,
		TemplateName:    req.TemplateName,
		TemplateContent: req.TemplateContent,
		Remark:          req.Remark,
		Status:          1,
	}

	if req.Status != nil {
		template.Status = *req.Status
	}

	if req.Variables != nil {
		if err := template.SetVariables(req.Variables); err != nil {
			return nil, fmt.Errorf("failed to set variables: %w", err)
		}
	}

	if err := s.providerTemplateDAO.Create(template); err != nil {
		return nil, fmt.Errorf("failed to create provider template: %w", err)
	}

	// 重新加载以获取关联数据
	template, err = s.providerTemplateDAO.GetByID(template.ID)
	if err != nil {
		return nil, err
	}

	return s.buildProviderTemplateResponse(template)
}

// UpdateProviderTemplate 更新供应商模板
func (s *TemplateService) UpdateProviderTemplate(id uint, req *dto.UpdateProviderTemplateRequest) (*dto.ProviderTemplateResponse, error) {
	// 获取现有模板
	template, err := s.providerTemplateDAO.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("provider template not found")
		}
		return nil, fmt.Errorf("failed to get provider template: %w", err)
	}

	// 更新字段
	if req.TemplateName != "" {
		template.TemplateName = req.TemplateName
	}
	if req.TemplateContent != "" {
		template.TemplateContent = req.TemplateContent
	}
	if req.Remark != "" {
		template.Remark = req.Remark
	}
	if req.Status != nil {
		template.Status = *req.Status
	}
	if req.Variables != nil {
		if err := template.SetVariables(req.Variables); err != nil {
			return nil, fmt.Errorf("failed to set variables: %w", err)
		}
	}

	if err := s.providerTemplateDAO.Update(template); err != nil {
		return nil, fmt.Errorf("failed to update provider template: %w", err)
	}

	return s.buildProviderTemplateResponse(template)
}

// GetProviderTemplate 获取供应商模板
func (s *TemplateService) GetProviderTemplate(id uint) (*dto.ProviderTemplateResponse, error) {
	template, err := s.providerTemplateDAO.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("provider template not found")
		}
		return nil, fmt.Errorf("failed to get provider template: %w", err)
	}

	return s.buildProviderTemplateResponse(template)
}

// DeleteProviderTemplate 删除供应商模板
func (s *TemplateService) DeleteProviderTemplate(id uint) error {
	return s.providerTemplateDAO.Delete(id)
}

// ListProviderTemplates 查询供应商模板列表
func (s *TemplateService) ListProviderTemplates(req *dto.ProviderTemplateListRequest) (*dto.ProviderTemplateListResponse, error) {
	templates, total, err := s.providerTemplateDAO.List(req.ProviderID, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list provider templates: %w", err)
	}

	list := make([]*dto.ProviderTemplateResponse, 0, len(templates))
	for _, template := range templates {
		resp, err := s.buildProviderTemplateResponse(template)
		if err != nil {
			continue
		}
		list = append(list, resp)
	}

	return &dto.ProviderTemplateListResponse{
		Items: list,
		Total: total,
		Page:  req.Page,
		Size:  req.PageSize,
	}, nil
}

// ========== 辅助方法 ==========

// buildMessageTemplateResponse 构建系统模板响应
func (s *TemplateService) buildMessageTemplateResponse(template *model.MessageTemplate) (*dto.MessageTemplateResponse, error) {
	variables, err := template.GetVariables()
	if err != nil {
		variables = []string{}
	}

	return &dto.MessageTemplateResponse{
		ID:           template.ID,
		TemplateCode: template.TemplateCode,
		TemplateName: template.TemplateName,
		MessageType:  template.MessageType,
		Content:      template.Content,
		Variables:    variables,
		Description:  template.Description,
		Status:       template.Status,
		CreatedAt:    template.CreatedAt,
		UpdatedAt:    template.UpdatedAt,
	}, nil
}

// buildProviderTemplateResponse 构建供应商模板响应
func (s *TemplateService) buildProviderTemplateResponse(template *model.ProviderTemplate) (*dto.ProviderTemplateResponse, error) {
	variables, err := template.GetVariables()
	if err != nil {
		variables = []string{}
	}

	resp := &dto.ProviderTemplateResponse{
		ID:              template.ID,
		ProviderID:      template.ProviderID,
		TemplateCode:    template.TemplateCode,
		TemplateName:    template.TemplateName,
		TemplateContent: template.TemplateContent,
		Variables:       variables,
		Status:          template.Status,
		Remark:          template.Remark,
		CreatedAt:       template.CreatedAt,
		UpdatedAt:       template.UpdatedAt,
	}

	if template.ProviderAccount != nil {
		resp.ProviderAccount = &dto.SimpleProviderResponse{
			ID:           template.ProviderAccount.ID,
			AccountCode:  template.ProviderAccount.AccountCode,
			AccountName:  template.ProviderAccount.AccountName,
			ProviderCode: template.ProviderAccount.ProviderCode,
			ProviderType: template.ProviderAccount.ProviderType,
		}
	}

	return resp, nil
}
