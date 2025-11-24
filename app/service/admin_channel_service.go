package service

import (
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// AdminChannelService 通道管理服务
type AdminChannelService struct {
	bindingDAO         *dao.ChannelTemplateBindingDAO
	templateBindingDAO *dao.TemplateBindingDAO
	messageTemplateDAO *dao.MessageTemplateDAO
}

// NewAdminChannelService 创建通道管理服务实例
func NewAdminChannelService() *AdminChannelService {
	return &AdminChannelService{
		bindingDAO:         dao.NewChannelTemplateBindingDAO(),
		templateBindingDAO: dao.NewTemplateBindingDAO(),
		messageTemplateDAO: dao.NewMessageTemplateDAO(),
	}
}

// CreateChannel 创建通道
func (s *AdminChannelService) CreateChannel(req *dto.CreateChannelRequest) (*dto.ChannelResponse, error) {
	logger := helper.GetHelper().GetLogger()
	db := helper.GetHelper().GetDatabase()

	// 验证系统模板是否存在
	messageTemplate, err := s.messageTemplateDAO.GetByID(req.MessageTemplateID)
	if err != nil {
		logger.Error("系统模板不存在")
		return nil, fmt.Errorf("message template not found: %w", err)
	}

	// 验证通道类型与模板类型是否匹配
	if messageTemplate.MessageType != req.Type {
		return nil, fmt.Errorf("channel type '%s' does not match template type '%s'", req.Type, messageTemplate.MessageType)
	}

	status := int8(req.Status)
	if status == 0 {
		status = 1 // 默认启用
	}

	channel := &model.Channel{
		Name:              req.Name,
		Type:              req.Type,
		MessageTemplateID: req.MessageTemplateID,
		Status:            status,
	}

	// 开启事务
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 创建通道
	if err := tx.Create(channel).Error; err != nil {
		tx.Rollback()
		logger.Error("创建通道失败")
		return nil, err
	}

	// 查询该系统模板的所有模板绑定
	templateBindings, err := s.templateBindingDAO.GetByMessageTemplateID(req.MessageTemplateID)
	if err != nil {
		tx.Rollback()
		logger.Error("查询模板绑定失败")
		return nil, fmt.Errorf("failed to get template bindings: %w", err)
	}

	// 为每个模板绑定创建默认的通道模板绑定配置
	if len(templateBindings) > 0 {
		channelBindings := make([]*model.ChannelTemplateBinding, 0, len(templateBindings))
		for _, tb := range templateBindings {
			channelBindings = append(channelBindings, &model.ChannelTemplateBinding{
				ChannelID:            channel.ID,
				TemplateBindingID:    tb.ID,
				Weight:               10, // 默认权重
				Priority:             tb.Priority,
				IsActive:             tb.Status,
				AutoDisableOnFail:    false,
				AutoDisableThreshold: 5,
			})
		}

		if err := tx.Create(&channelBindings).Error; err != nil {
			tx.Rollback()
			logger.Error("创建通道绑定配置失败")
			return nil, fmt.Errorf("failed to create channel bindings: %w", err)
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		logger.Error("提交事务失败")
		return nil, err
	}

	logger.Info("通道创建成功")

	return &dto.ChannelResponse{
		ID:                channel.ID,
		Name:              channel.Name,
		Type:              channel.Type,
		MessageTemplateID: channel.MessageTemplateID,
		TemplateName:      messageTemplate.TemplateName,
		Status:            int(channel.Status),
		CreatedAt:         channel.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         channel.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// GetChannelList 获取通道列表
func (s *AdminChannelService) GetChannelList(req *dto.ChannelListRequest) (*dto.ChannelListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	var channels []*model.Channel
	var total int64

	query := helper.GetHelper().GetDatabase().Model(&model.Channel{}).Preload("MessageTemplate")

	// 条件过滤
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	if req.Status > 0 {
		query = query.Where("status = ?", req.Status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count channels: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}

	items := make([]*dto.ChannelResponse, 0, len(channels))
	for _, channel := range channels {
		item := &dto.ChannelResponse{
			ID:                channel.ID,
			Name:              channel.Name,
			Type:              channel.Type,
			MessageTemplateID: channel.MessageTemplateID,
			Status:            int(channel.Status),
			CreatedAt:         channel.CreatedAt.Format(time.RFC3339),
			UpdatedAt:         channel.UpdatedAt.Format(time.RFC3339),
		}
		if channel.MessageTemplate != nil {
			item.TemplateName = channel.MessageTemplate.TemplateName
		}
		items = append(items, item)
	}

	return &dto.ChannelListResponse{
		Total: int(total),
		Page:  page,
		Size:  pageSize,
		Items: items,
	}, nil
}

// GetChannelByID 获取通道详情
func (s *AdminChannelService) GetChannelByID(id uint) (*dto.ChannelResponse, error) {
	var channel model.Channel
	db := helper.GetHelper().GetDatabase()
	if err := db.Preload("MessageTemplate").First(&channel, id).Error; err != nil {
		return nil, err
	}

	// 获取绑定配置
	bindings, err := s.GetChannelBindings(id)
	if err != nil {
		return nil, err
	}

	response := &dto.ChannelResponse{
		ID:                channel.ID,
		Name:              channel.Name,
		Type:              channel.Type,
		MessageTemplateID: channel.MessageTemplateID,
		Status:            int(channel.Status),
		CreatedAt:         channel.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         channel.UpdatedAt.Format(time.RFC3339),
		Bindings:          bindings,
	}
	if channel.MessageTemplate != nil {
		response.TemplateName = channel.MessageTemplate.TemplateName
	}

	return response, nil
}

// UpdateChannel 更新通道
func (s *AdminChannelService) UpdateChannel(id uint, req *dto.UpdateChannelRequest) error {
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Status > 0 {
		updates["status"] = int8(req.Status)
	}

	if len(updates) == 0 {
		return nil
	}

	db := helper.GetHelper().GetDatabase()
	return db.Model(&model.Channel{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteChannel 删除通道
func (s *AdminChannelService) DeleteChannel(id uint) error {
	db := helper.GetHelper().GetDatabase()

	// 开启事务
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 删除通道的所有绑定配置
	if err := s.bindingDAO.DeleteByChannelID(id); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to delete channel bindings: %w", err)
	}

	// 删除通道
	if err := tx.Delete(&model.Channel{}, id).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return err
	}

	return nil
}

// GetChannelBindings 获取通道的模板绑定配置列表
func (s *AdminChannelService) GetChannelBindings(channelID uint) ([]*dto.ChannelBindingResponse, error) {
	bindings, err := s.bindingDAO.GetByChannelID(channelID)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.ChannelBindingResponse, 0, len(bindings))
	for _, b := range bindings {
		item := &dto.ChannelBindingResponse{
			ID:                   b.ID,
			TemplateBindingID:    b.TemplateBindingID,
			Weight:               b.Weight,
			Priority:             b.Priority,
			IsActive:             b.IsActive,
			AutoDisableOnFail:    b.AutoDisableOnFail,
			AutoDisableThreshold: b.AutoDisableThreshold,
			CreatedAt:            b.CreatedAt.Format(time.RFC3339),
		}

		if b.TemplateBinding != nil {
			if b.TemplateBinding.ProviderTemplate != nil {
				item.ProviderTemplateID = b.TemplateBinding.ProviderTemplate.ID
				item.ProviderTemplateName = b.TemplateBinding.ProviderTemplate.TemplateName
				item.ProviderID = b.TemplateBinding.ProviderTemplate.ProviderID

				if b.TemplateBinding.ProviderTemplate.ProviderAccount != nil {
					item.ProviderName = b.TemplateBinding.ProviderTemplate.ProviderAccount.AccountName
					item.ProviderType = b.TemplateBinding.ProviderTemplate.ProviderAccount.ProviderType
				}
			}
		}

		items = append(items, item)
	}

	return items, nil
}

// UpdateChannelBinding 更新通道绑定配置
func (s *AdminChannelService) UpdateChannelBinding(bindingID uint, req *dto.UpdateChannelBindingRequest) error {
	// 检查绑定是否存在
	_, err := s.bindingDAO.GetByID(bindingID)
	if err != nil {
		return fmt.Errorf("binding not found: %w", err)
	}

	updates := make(map[string]interface{})

	if req.Weight > 0 {
		updates["weight"] = req.Weight
	}
	if req.Priority >= 0 {
		updates["priority"] = req.Priority
	}
	if req.IsActive >= 0 {
		updates["is_active"] = req.IsActive
	}
	updates["auto_disable_on_fail"] = req.AutoDisableOnFail
	if req.AutoDisableThreshold > 0 {
		updates["auto_disable_threshold"] = req.AutoDisableThreshold
	}

	if len(updates) == 0 {
		return nil
	}

	return s.bindingDAO.Update(bindingID, updates)
}

// GetActiveChannels 获取活跃通道列表
func (s *AdminChannelService) GetActiveChannels() ([]*dto.ActiveItem, error) {
	var channels []*model.Channel
	db := helper.GetHelper().GetDatabase()
	if err := db.Where("status = ?", 1).Find(&channels).Error; err != nil {
		return nil, err
	}

	items := make([]*dto.ActiveItem, 0, len(channels))
	for _, c := range channels {
		items = append(items, &dto.ActiveItem{
			ID:   c.ID,
			Name: c.Name,
			Type: c.Type,
		})
	}
	return items, nil
}

// CreateChannelBinding 创建通道绑定配置
func (s *AdminChannelService) CreateChannelBinding(channelID uint, req *dto.CreateChannelBindingRequest) (*dto.ChannelBindingResponse, error) {
	logger := helper.GetHelper().GetLogger()
	db := helper.GetHelper().GetDatabase()

	// 验证通道是否存在
	var channel model.Channel
	if err := db.First(&channel, channelID).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	// 验证模板绑定是否存在
	templateBinding, err := s.templateBindingDAO.GetByID(req.TemplateBindingID)
	if err != nil {
		return nil, fmt.Errorf("template binding not found: %w", err)
	}

	// 验证模板绑定是否属于该通道的系统模板
	if templateBinding.MessageTemplateID != channel.MessageTemplateID {
		return nil, fmt.Errorf("template binding does not belong to channel's message template")
	}

	// 检查是否已经存在相同的绑定
	existingBindings, err := s.bindingDAO.GetByChannelID(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing bindings: %w", err)
	}
	for _, b := range existingBindings {
		if b.TemplateBindingID == req.TemplateBindingID {
			return nil, fmt.Errorf("binding already exists for this template binding")
		}
	}

	// 设置默认值
	weight := req.Weight
	if weight == 0 {
		weight = 10
	}
	priority := req.Priority
	if priority == 0 {
		priority = 100
	}
	isActive := req.IsActive
	if isActive == 0 {
		isActive = 1
	}
	autoDisableThreshold := req.AutoDisableThreshold
	if autoDisableThreshold == 0 {
		autoDisableThreshold = 5
	}

	// 创建通道绑定配置
	binding := &model.ChannelTemplateBinding{
		ChannelID:            channelID,
		TemplateBindingID:    req.TemplateBindingID,
		Weight:               weight,
		Priority:             priority,
		IsActive:             isActive,
		AutoDisableOnFail:    req.AutoDisableOnFail,
		AutoDisableThreshold: autoDisableThreshold,
	}

	if err := db.Create(binding).Error; err != nil {
		logger.Error("创建通道绑定配置失败")
		return nil, fmt.Errorf("failed to create channel binding: %w", err)
	}

	// 预加载关联数据
	if err := db.Preload("TemplateBinding.ProviderTemplate.ProviderAccount").First(binding, binding.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to load binding details: %w", err)
	}

	// 构建响应
	response := &dto.ChannelBindingResponse{
		ID:                   binding.ID,
		TemplateBindingID:    binding.TemplateBindingID,
		Weight:               binding.Weight,
		Priority:             binding.Priority,
		IsActive:             binding.IsActive,
		AutoDisableOnFail:    binding.AutoDisableOnFail,
		AutoDisableThreshold: binding.AutoDisableThreshold,
		CreatedAt:            binding.CreatedAt.Format(time.RFC3339),
	}

	if binding.TemplateBinding != nil {
		if binding.TemplateBinding.ProviderTemplate != nil {
			response.ProviderTemplateID = binding.TemplateBinding.ProviderTemplate.ID
			response.ProviderTemplateName = binding.TemplateBinding.ProviderTemplate.TemplateName
			response.ProviderID = binding.TemplateBinding.ProviderTemplate.ProviderID

			if binding.TemplateBinding.ProviderTemplate.ProviderAccount != nil {
				response.ProviderName = binding.TemplateBinding.ProviderTemplate.ProviderAccount.AccountName
				response.ProviderType = binding.TemplateBinding.ProviderTemplate.ProviderAccount.ProviderType
			}
		}
	}

	logger.Info("通道绑定配置创建成功")
	return response, nil
}

// GetAvailableTemplateBindings 获取通道可用的模板绑定列表（排除已添加的）
func (s *AdminChannelService) GetAvailableTemplateBindings(channelID uint) ([]*dto.AvailableTemplateBindingResponse, error) {
	db := helper.GetHelper().GetDatabase()

	// 验证通道是否存在
	var channel model.Channel
	if err := db.First(&channel, channelID).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	// 获取该通道系统模板的所有模板绑定
	allBindings, err := s.templateBindingDAO.GetByMessageTemplateID(channel.MessageTemplateID)
	if err != nil {
		return nil, fmt.Errorf("failed to get template bindings: %w", err)
	}

	// 获取该通道已添加的绑定
	existingBindings, err := s.bindingDAO.GetByChannelID(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing bindings: %w", err)
	}

	// 构建已存在的模板绑定ID集合
	existingMap := make(map[uint]bool)
	for _, b := range existingBindings {
		existingMap[b.TemplateBindingID] = true
	}

	// 过滤出可用的模板绑定
	items := make([]*dto.AvailableTemplateBindingResponse, 0)
	for _, tb := range allBindings {
		if existingMap[tb.ID] {
			continue // 跳过已添加的绑定
		}

		// 预加载关联数据
		if err := db.Preload("ProviderTemplate.ProviderAccount").First(tb, tb.ID).Error; err != nil {
			continue
		}

		item := &dto.AvailableTemplateBindingResponse{
			ID:       tb.ID,
			Priority: tb.Priority,
			Status:   tb.Status,
		}

		if tb.ProviderTemplate != nil {
			item.ProviderTemplateID = tb.ProviderTemplate.ID
			item.ProviderTemplateName = tb.ProviderTemplate.TemplateName
			item.ProviderID = tb.ProviderTemplate.ProviderID

			if tb.ProviderTemplate.ProviderAccount != nil {
				item.ProviderName = tb.ProviderTemplate.ProviderAccount.AccountName
				item.ProviderType = tb.ProviderTemplate.ProviderAccount.ProviderType
			}
		}

		items = append(items, item)
	}

	return items, nil
}
