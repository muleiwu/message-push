package service

import (
	"context"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/sender"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// AdminProviderService 服务商管理服务
type AdminProviderService struct{}

// NewAdminProviderService 创建服务商管理服务实例
func NewAdminProviderService() *AdminProviderService {
	return &AdminProviderService{}
}

// CreateProvider 创建服务商
func (s *AdminProviderService) CreateProvider(req *dto.CreateProviderRequest) (*dto.ProviderResponse, error) {
	logger := helper.GetHelper().GetLogger()

	status := int8(req.Status)
	if status == 0 {
		status = 1 // 默认启用
	}

	// 生成provider_code
	providerCode, err := generateRandomKey(8)
	if err != nil {
		logger.Error("生成provider_code失败")
		return nil, fmt.Errorf("failed to generate provider_code: %w", err)
	}

	provider := &model.Provider{
		ProviderCode: providerCode,
		ProviderName: req.Name,
		ProviderType: req.Type,
		Status:       status,
		Remark:       req.Description,
	}

	// 设置配置
	if err := provider.SetConfig(req.Config); err != nil {
		logger.Error("设置服务商配置失败")
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	db := helper.GetHelper().GetDatabase()
	if err := db.Create(provider).Error; err != nil {
		logger.Error("创建服务商失败")
		return nil, err
	}

	logger.Info("服务商创建成功")

	config, _ := provider.GetConfig()
	return &dto.ProviderResponse{
		ID:          provider.ID,
		Name:        provider.ProviderName,
		Type:        provider.ProviderType,
		Description: req.Description,
		Config:      config,
		Status:      int(provider.Status),
		CreatedAt:   provider.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   provider.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// GetProviderList 获取服务商列表
func (s *AdminProviderService) GetProviderList(req *dto.ProviderListRequest) (*dto.ProviderListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	var providers []*model.Provider
	var total int64

	query := helper.GetHelper().GetDatabase().Model(&model.Provider{})

	// 条件过滤
	if req.Type != "" {
		query = query.Where("provider_type = ?", req.Type)
	}
	if req.Status > 0 {
		query = query.Where("status = ?", req.Status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count providers: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("failed to query providers: %w", err)
	}

	items := make([]*dto.ProviderResponse, 0, len(providers))
	for _, provider := range providers {
		config, _ := provider.GetConfig()
		items = append(items, &dto.ProviderResponse{
			ID:          provider.ID,
			Name:        provider.ProviderName,
			Type:        provider.ProviderType,
			Description: provider.Remark,
			Config:      config,
			Status:      int(provider.Status),
			CreatedAt:   provider.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   provider.UpdatedAt.Format(time.RFC3339),
		})
	}

	return &dto.ProviderListResponse{
		Total: int(total),
		Page:  page,
		Size:  pageSize,
		Items: items,
	}, nil
}

// GetProviderByID 获取服务商详情
func (s *AdminProviderService) GetProviderByID(id uint) (*dto.ProviderResponse, error) {
	var provider model.Provider
	db := helper.GetHelper().GetDatabase()
	if err := db.First(&provider, id).Error; err != nil {
		return nil, err
	}

	config, _ := provider.GetConfig()
	return &dto.ProviderResponse{
		ID:          provider.ID,
		Name:        provider.ProviderName,
		Type:        provider.ProviderType,
		Description: provider.Remark,
		Config:      config,
		Status:      int(provider.Status),
		CreatedAt:   provider.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   provider.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// UpdateProvider 更新服务商
func (s *AdminProviderService) UpdateProvider(id uint, req *dto.UpdateProviderRequest) error {
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["provider_name"] = req.Name
	}
	if req.Description != "" {
		updates["remark"] = req.Description
	}
	if req.Status > 0 {
		updates["status"] = int8(req.Status)
	}
	if len(req.Config) > 0 {
		// 需要先查询现有Provider来处理配置
		var provider model.Provider
		db := helper.GetHelper().GetDatabase()
		if err := db.First(&provider, id).Error; err != nil {
			return err
		}
		if err := provider.SetConfig(req.Config); err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
		updates["config"] = provider.Config
	}

	if len(updates) == 0 {
		return nil
	}

	db := helper.GetHelper().GetDatabase()
	return db.Model(&model.Provider{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteProvider 删除服务商
func (s *AdminProviderService) DeleteProvider(id uint) error {
	db := helper.GetHelper().GetDatabase()
	return db.Delete(&model.Provider{}, id).Error
}

// GetActiveProviders 获取活跃服务商列表
func (s *AdminProviderService) GetActiveProviders(providerType string) ([]*dto.ActiveItem, error) {
	var providers []*model.Provider
	query := helper.GetHelper().GetDatabase().Where("status = ?", 1)

	if providerType != "" {
		query = query.Where("provider_type = ?", providerType)
	}

	if err := query.Find(&providers).Error; err != nil {
		return nil, err
	}

	items := make([]*dto.ActiveItem, 0, len(providers))
	for _, p := range providers {
		items = append(items, &dto.ActiveItem{
			ID:           p.ID,
			ProviderCode: p.ProviderCode,
			ProviderName: p.ProviderName,
			ProviderType: p.ProviderType,
		})
	}
	return items, nil
}

// TestProvider 测试服务商配置
func (s *AdminProviderService) TestProvider(id uint, req *dto.TestProviderRequest) (*dto.TestProviderResponse, error) {
	// 1. 获取服务商配置
	var provider model.Provider
	db := helper.GetHelper().GetDatabase()
	if err := db.First(&provider, id).Error; err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}

	// 2. 构造发送请求
	task := &model.PushTask{
		MessageType: provider.ProviderType, // 简单映射
	}

	// 根据类型填充接收者
	if provider.ProviderType == "sms" {
		if req.Phone == "" {
			return nil, fmt.Errorf("phone number is required for sms")
		}
		task.Receiver = req.Phone
		task.Content = req.Message // 简单透传，部分sender可能需要模板ID
	} else if provider.ProviderType == "email" {
		if req.Email == "" {
			return nil, fmt.Errorf("email is required for email")
		}
		task.Receiver = req.Email
		task.Content = req.Message
		task.Title = "测试邮件"
		// Email通常需要Title，这里简化
	} else {
		// 其他类型暂不支持测试或使用Message字段
		task.Content = req.Message
	}

	// 将 Provider 转换为 ProviderAccount 以适配新接口
	providerAccount := &model.ProviderAccount{
		ID:           provider.ID,
		AccountCode:  provider.ProviderCode,
		AccountName:  provider.ProviderName,
		ProviderCode: provider.ProviderCode,
		ProviderType: provider.ProviderType,
		Config:       provider.Config,
		Status:       provider.Status,
	}

	sendReq := &sender.SendRequest{
		Task:            task,
		ProviderAccount: providerAccount,
		ProviderChannel: &model.ProviderChannel{
			// 构造一个临时的ProviderChannel
			ProviderID: provider.ID,
			Config:     provider.Config,
		},
	}

	// 3. 获取发送器（使用服务商代码）
	factory := sender.NewFactory()
	msgSender, err := factory.GetSender(provider.ProviderCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender: %w", err)
	}

	// 4. 发送测试消息
	// 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := msgSender.Send(ctx, sendReq)
	if err != nil {
		return &dto.TestProviderResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &dto.TestProviderResponse{
		Success: resp.Success,
		Message: resp.ErrorMessage,
	}, nil
}
