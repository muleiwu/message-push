package service

import (
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/selector"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// convertModelParamMappingToDTO 将 model.ParamMappingItem 转换为 dto.ParamMappingItem
func convertModelParamMappingToDTO(items []model.ParamMappingItem) []dto.ParamMappingItem {
	if items == nil {
		return nil
	}
	result := make([]dto.ParamMappingItem, len(items))
	for i, item := range items {
		result[i] = dto.ParamMappingItem{
			Type:        dto.ParamMappingType(item.Type),
			ProviderVar: item.ProviderVar,
			SystemVar:   item.SystemVar,
			Value:       item.Value,
		}
	}
	return result
}

// convertDTOParamMappingToModel 将 dto.ParamMappingItem 转换为 model.ParamMappingItem
func convertDTOParamMappingToModel(items []dto.ParamMappingItem) []model.ParamMappingItem {
	if items == nil {
		return nil
	}
	result := make([]model.ParamMappingItem, len(items))
	for i, item := range items {
		result[i] = model.ParamMappingItem{
			Type:        model.ParamMappingType(item.Type),
			ProviderVar: item.ProviderVar,
			SystemVar:   item.SystemVar,
			Value:       item.Value,
		}
	}
	return result
}

// AdminChannelService 通道管理服务
type AdminChannelService struct {
	bindingDAO           *dao.ChannelTemplateBindingDAO
	messageTemplateDAO   *dao.MessageTemplateDAO
	providerTemplateDAO  *dao.ProviderTemplateDAO
	signatureMappingDAO  *dao.ChannelSignatureMappingDAO
	providerSignatureDAO *dao.ProviderSignatureDAO
	channelSelector      *selector.ChannelSelector // 用于在配置变更时重置缓存和权重
}

// NewAdminChannelService 创建通道管理服务实例
func NewAdminChannelService() *AdminChannelService {
	db := helper.GetHelper().GetDatabase()
	return &AdminChannelService{
		bindingDAO:           dao.NewChannelTemplateBindingDAO(),
		messageTemplateDAO:   dao.NewMessageTemplateDAO(),
		providerTemplateDAO:  dao.NewProviderTemplateDAO(),
		signatureMappingDAO:  dao.NewChannelSignatureMappingDAO(db),
		providerSignatureDAO: dao.NewProviderSignatureDAO(db),
		channelSelector:      selector.NewChannelSelector(),
	}
}

// invalidateChannelCache 清除通道缓存和权重状态
// 在通道配置变更后调用，确保变更立即生效
func (s *AdminChannelService) invalidateChannelCache(channelID uint) {
	s.channelSelector.InvalidateCacheForBinding(channelID)
	s.channelSelector.ResetWeightsByChannelID(channelID)
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

	// 创建通道（不再自动创建绑定，由用户手动配置）
	if err := db.Create(channel).Error; err != nil {
		logger.Error("创建通道失败")
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

	// 清除通道缓存和权重状态
	s.invalidateChannelCache(id)

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
		// 获取参数映射
		paramMapping, _ := b.GetParamMapping()

		item := &dto.ChannelBindingResponse{
			ID:                   b.ID,
			ProviderTemplateID:   b.ProviderTemplateID,
			ProviderID:           b.ProviderID,
			ParamMapping:         convertModelParamMappingToDTO(paramMapping),
			Weight:               b.Weight,
			Priority:             b.Priority,
			Status:               b.Status,
			IsActive:             b.IsActive,
			AutoDisableOnFail:    b.AutoDisableOnFail,
			AutoDisableThreshold: b.AutoDisableThreshold,
			CreatedAt:            b.CreatedAt.Format(time.RFC3339),
		}

		if b.ProviderTemplate != nil {
			item.ProviderTemplateName = b.ProviderTemplate.TemplateName

			if b.ProviderTemplate.ProviderAccount != nil {
				item.ProviderName = b.ProviderTemplate.ProviderAccount.AccountName
				item.ProviderType = b.ProviderTemplate.ProviderAccount.ProviderType
			}
		}

		items = append(items, item)
	}

	return items, nil
}

// UpdateChannelBinding 更新通道绑定配置
func (s *AdminChannelService) UpdateChannelBinding(bindingID uint, req *dto.UpdateChannelBindingRequest) error {
	// 检查绑定是否存在
	binding, err := s.bindingDAO.GetByID(bindingID)
	if err != nil {
		return fmt.Errorf("binding not found: %w", err)
	}

	updates := make(map[string]interface{})

	if req.ParamMapping != nil {
		if err := binding.SetParamMapping(convertDTOParamMappingToModel(req.ParamMapping)); err != nil {
			return fmt.Errorf("failed to set param mapping: %w", err)
		}
		updates["param_mapping"] = binding.ParamMapping
	}
	if req.Weight > 0 {
		updates["weight"] = req.Weight
	}
	if req.Priority >= 0 {
		updates["priority"] = req.Priority
	}
	if req.Status >= 0 {
		updates["status"] = req.Status
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

	if err := s.bindingDAO.Update(bindingID, updates); err != nil {
		return err
	}

	// 清除通道缓存和权重状态，确保配置变更立即生效
	s.invalidateChannelCache(binding.ChannelID)

	return nil
}

// DeleteChannelBinding 删除通道绑定配置
func (s *AdminChannelService) DeleteChannelBinding(bindingID uint) error {
	// 检查绑定是否存在
	binding, err := s.bindingDAO.GetByID(bindingID)
	if err != nil {
		return fmt.Errorf("binding not found: %w", err)
	}

	channelID := binding.ChannelID

	// 删除绑定
	if err := s.bindingDAO.Delete(bindingID); err != nil {
		return err
	}

	// 清除通道缓存和权重状态，确保配置变更立即生效
	s.invalidateChannelCache(channelID)

	return nil
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

// GetChannelBinding 获取单个通道绑定配置
func (s *AdminChannelService) GetChannelBinding(bindingID uint) (*dto.ChannelBindingResponse, error) {
	binding, err := s.bindingDAO.GetByID(bindingID)
	if err != nil {
		return nil, fmt.Errorf("binding not found: %w", err)
	}

	// 获取参数映射
	paramMapping, _ := binding.GetParamMapping()

	item := &dto.ChannelBindingResponse{
		ID:                   binding.ID,
		ProviderTemplateID:   binding.ProviderTemplateID,
		ProviderID:           binding.ProviderID,
		ParamMapping:         convertModelParamMappingToDTO(paramMapping),
		Weight:               binding.Weight,
		Priority:             binding.Priority,
		Status:               binding.Status,
		IsActive:             binding.IsActive,
		AutoDisableOnFail:    binding.AutoDisableOnFail,
		AutoDisableThreshold: binding.AutoDisableThreshold,
		CreatedAt:            binding.CreatedAt.Format(time.RFC3339),
	}

	if binding.ProviderTemplate != nil {
		item.ProviderTemplateName = binding.ProviderTemplate.TemplateName

		if binding.ProviderTemplate.ProviderAccount != nil {
			item.ProviderName = binding.ProviderTemplate.ProviderAccount.AccountName
			item.ProviderType = binding.ProviderTemplate.ProviderAccount.ProviderType
		}
	}

	return item, nil
}

// CreateChannelBinding 创建通道绑定配置
func (s *AdminChannelService) CreateChannelBinding(channelID uint, req *dto.CreateChannelBindingRequest) (*dto.ChannelBindingResponse, error) {
	logger := helper.GetHelper().GetLogger()
	db := helper.GetHelper().GetDatabase()

	// 验证通道是否存在
	var channel model.Channel
	if err := db.Preload("MessageTemplate").First(&channel, channelID).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	// 验证供应商模板是否存在
	providerTemplate, err := s.providerTemplateDAO.GetByID(req.ProviderTemplateID)
	if err != nil {
		return nil, fmt.Errorf("provider template not found: %w", err)
	}

	// 验证供应商账号是否存在且匹配
	if providerTemplate.ProviderID != req.ProviderID {
		return nil, fmt.Errorf("provider template does not belong to the specified provider account")
	}

	// 验证供应商类型是否与通道类型匹配
	if providerTemplate.ProviderAccount != nil {
		if providerTemplate.ProviderAccount.ProviderType != channel.Type {
			return nil, fmt.Errorf("provider type '%s' does not match channel type '%s'",
				providerTemplate.ProviderAccount.ProviderType, channel.Type)
		}
	}

	// 检查是否已经存在相同的绑定
	existingBindings, err := s.bindingDAO.GetByChannelID(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing bindings: %w", err)
	}
	for _, b := range existingBindings {
		if b.ProviderTemplateID == req.ProviderTemplateID {
			return nil, fmt.Errorf("binding already exists for this provider template")
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
	status := req.Status
	if status == 0 {
		status = 1
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
		ProviderTemplateID:   req.ProviderTemplateID,
		ProviderID:           req.ProviderID,
		Weight:               weight,
		Priority:             priority,
		Status:               status,
		IsActive:             isActive,
		AutoDisableOnFail:    req.AutoDisableOnFail,
		AutoDisableThreshold: autoDisableThreshold,
	}

	// 设置参数映射
	if req.ParamMapping != nil {
		if err := binding.SetParamMapping(convertDTOParamMappingToModel(req.ParamMapping)); err != nil {
			return nil, fmt.Errorf("failed to set param mapping: %w", err)
		}
	}

	if err := db.Create(binding).Error; err != nil {
		logger.Error("创建通道绑定配置失败")
		return nil, fmt.Errorf("failed to create channel binding: %w", err)
	}

	// 预加载关联数据
	if err := db.Preload("ProviderTemplate.ProviderAccount").First(binding, binding.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to load binding details: %w", err)
	}

	// 获取参数映射
	paramMapping, _ := binding.GetParamMapping()

	// 构建响应
	response := &dto.ChannelBindingResponse{
		ID:                   binding.ID,
		ProviderTemplateID:   binding.ProviderTemplateID,
		ProviderID:           binding.ProviderID,
		ParamMapping:         convertModelParamMappingToDTO(paramMapping),
		Weight:               binding.Weight,
		Priority:             binding.Priority,
		Status:               binding.Status,
		IsActive:             binding.IsActive,
		AutoDisableOnFail:    binding.AutoDisableOnFail,
		AutoDisableThreshold: binding.AutoDisableThreshold,
		CreatedAt:            binding.CreatedAt.Format(time.RFC3339),
	}

	if binding.ProviderTemplate != nil {
		response.ProviderTemplateName = binding.ProviderTemplate.TemplateName

		if binding.ProviderTemplate.ProviderAccount != nil {
			response.ProviderName = binding.ProviderTemplate.ProviderAccount.AccountName
			response.ProviderType = binding.ProviderTemplate.ProviderAccount.ProviderType
		}
	}

	// 清除通道缓存和权重状态，确保新配置立即生效
	s.invalidateChannelCache(channelID)

	logger.Info("通道绑定配置创建成功")
	return response, nil
}

// GetAvailableTemplateBindings 获取通道可用的供应商模板列表（排除已添加的）
func (s *AdminChannelService) GetAvailableTemplateBindings(channelID uint) ([]*dto.AvailableProviderTemplateResponse, error) {
	db := helper.GetHelper().GetDatabase()

	// 验证通道是否存在
	var channel model.Channel
	if err := db.First(&channel, channelID).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	// 获取该通道已添加的绑定
	existingBindings, err := s.bindingDAO.GetByChannelID(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing bindings: %w", err)
	}

	// 构建已存在的供应商模板ID集合
	existingMap := make(map[uint]bool)
	for _, b := range existingBindings {
		existingMap[b.ProviderTemplateID] = true
	}

	// 获取所有匹配通道类型的供应商模板
	var allTemplates []*model.ProviderTemplate
	if err := db.Joins("JOIN provider_accounts ON provider_accounts.id = provider_templates.provider_id").
		Where("provider_accounts.provider_type = ? AND provider_accounts.status = 1 AND provider_templates.status = 1", channel.Type).
		Preload("ProviderAccount").
		Find(&allTemplates).Error; err != nil {
		return nil, fmt.Errorf("failed to get provider templates: %w", err)
	}

	// 过滤出可用的供应商模板
	items := make([]*dto.AvailableProviderTemplateResponse, 0)
	for _, pt := range allTemplates {
		if existingMap[pt.ID] {
			continue // 跳过已添加的
		}

		// 解析变量列表
		variables, _ := pt.GetVariables()

		item := &dto.AvailableProviderTemplateResponse{
			ID:              pt.ID,
			TemplateCode:    pt.TemplateCode,
			TemplateName:    pt.TemplateName,
			TemplateContent: pt.TemplateContent,
			Variables:       variables,
			ProviderID:      pt.ProviderID,
			Status:          pt.Status,
		}

		if pt.ProviderAccount != nil {
			item.ProviderAccountCode = pt.ProviderAccount.AccountCode
			item.ProviderAccountName = pt.ProviderAccount.AccountName
			item.ProviderCode = pt.ProviderAccount.ProviderCode
			item.ProviderType = pt.ProviderAccount.ProviderType
		}

		items = append(items, item)
	}

	return items, nil
}

// GetChannelSignatureMappings 获取通道签名映射列表
func (s *AdminChannelService) GetChannelSignatureMappings(channelID uint) ([]*dto.ChannelSignatureMappingResponse, error) {
	mappings, err := s.signatureMappingDAO.GetByChannelID(channelID)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.ChannelSignatureMappingResponse, 0, len(mappings))
	for _, m := range mappings {
		item := &dto.ChannelSignatureMappingResponse{
			ID:                  m.ID,
			ChannelID:           m.ChannelID,
			SignatureName:       m.SignatureName,
			ProviderSignatureID: m.ProviderSignatureID,
			ProviderID:          m.ProviderID,
			Status:              m.Status,
			CreatedAt:           m.CreatedAt.Format(time.RFC3339),
			UpdatedAt:           m.UpdatedAt.Format(time.RFC3339),
		}

		if m.ProviderSignature != nil {
			item.ProviderSignatureName = m.ProviderSignature.SignatureName
			item.ProviderSignatureCode = m.ProviderSignature.SignatureCode
		}

		if m.ProviderAccount != nil {
			item.ProviderAccountName = m.ProviderAccount.AccountName
			item.ProviderCode = m.ProviderAccount.ProviderCode
		}

		items = append(items, item)
	}

	return items, nil
}

// GetChannelSignatureMapping 获取单个签名映射
func (s *AdminChannelService) GetChannelSignatureMapping(mappingID uint) (*dto.ChannelSignatureMappingResponse, error) {
	mapping, err := s.signatureMappingDAO.GetByID(mappingID)
	if err != nil {
		return nil, fmt.Errorf("signature mapping not found: %w", err)
	}

	item := &dto.ChannelSignatureMappingResponse{
		ID:                  mapping.ID,
		ChannelID:           mapping.ChannelID,
		SignatureName:       mapping.SignatureName,
		ProviderSignatureID: mapping.ProviderSignatureID,
		ProviderID:          mapping.ProviderID,
		Status:              mapping.Status,
		CreatedAt:           mapping.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           mapping.UpdatedAt.Format(time.RFC3339),
	}

	if mapping.ProviderSignature != nil {
		item.ProviderSignatureName = mapping.ProviderSignature.SignatureName
		item.ProviderSignatureCode = mapping.ProviderSignature.SignatureCode
	}

	if mapping.ProviderAccount != nil {
		item.ProviderAccountName = mapping.ProviderAccount.AccountName
		item.ProviderCode = mapping.ProviderAccount.ProviderCode
	}

	return item, nil
}

// CreateChannelSignatureMapping 创建签名映射
func (s *AdminChannelService) CreateChannelSignatureMapping(channelID uint, req *dto.CreateChannelSignatureMappingRequest) (*dto.ChannelSignatureMappingResponse, error) {
	logger := helper.GetHelper().GetLogger()
	db := helper.GetHelper().GetDatabase()

	// 验证通道是否存在
	var channel model.Channel
	if err := db.First(&channel, channelID).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	// 验证供应商签名是否存在
	providerSignature, err := s.providerSignatureDAO.GetByID(req.ProviderSignatureID)
	if err != nil {
		return nil, fmt.Errorf("provider signature not found: %w", err)
	}

	// 验证供应商账号是否匹配
	if providerSignature.ProviderAccountID != req.ProviderID {
		return nil, fmt.Errorf("provider signature does not belong to the specified provider account")
	}

	// 检查签名名称+供应商是否已存在
	exists, err := s.signatureMappingDAO.CheckDuplicateSignatureName(channelID, req.SignatureName, req.ProviderID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to check duplicate signature name: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("signature name '%s' with this provider already exists in this channel", req.SignatureName)
	}

	// 设置默认值
	status := int8(1)
	if req.Status != nil {
		status = *req.Status
	}

	// 创建签名映射
	mapping := &model.ChannelSignatureMapping{
		ChannelID:           channelID,
		SignatureName:       req.SignatureName,
		ProviderSignatureID: req.ProviderSignatureID,
		ProviderID:          req.ProviderID,
		Status:              status,
	}

	if err := s.signatureMappingDAO.Create(mapping); err != nil {
		logger.Error("创建签名映射失败")
		return nil, fmt.Errorf("failed to create signature mapping: %w", err)
	}

	// 重新加载以获取关联数据
	mapping, err = s.signatureMappingDAO.GetByID(mapping.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load signature mapping: %w", err)
	}

	// 构建响应
	response := &dto.ChannelSignatureMappingResponse{
		ID:                  mapping.ID,
		ChannelID:           mapping.ChannelID,
		SignatureName:       mapping.SignatureName,
		ProviderSignatureID: mapping.ProviderSignatureID,
		ProviderID:          mapping.ProviderID,
		Status:              mapping.Status,
		CreatedAt:           mapping.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           mapping.UpdatedAt.Format(time.RFC3339),
	}

	if mapping.ProviderSignature != nil {
		response.ProviderSignatureName = mapping.ProviderSignature.SignatureName
		response.ProviderSignatureCode = mapping.ProviderSignature.SignatureCode
	}

	if mapping.ProviderAccount != nil {
		response.ProviderAccountName = mapping.ProviderAccount.AccountName
		response.ProviderCode = mapping.ProviderAccount.ProviderCode
	}

	logger.Info("签名映射创建成功")
	return response, nil
}

// UpdateChannelSignatureMapping 更新签名映射
func (s *AdminChannelService) UpdateChannelSignatureMapping(mappingID uint, req *dto.UpdateChannelSignatureMappingRequest) error {
	// 检查映射是否存在
	mapping, err := s.signatureMappingDAO.GetByID(mappingID)
	if err != nil {
		return fmt.Errorf("signature mapping not found: %w", err)
	}

	// 如果更新签名名称，检查是否重复
	if req.SignatureName != "" && req.SignatureName != mapping.SignatureName {
		exists, err := s.signatureMappingDAO.CheckDuplicateSignatureName(mapping.ChannelID, req.SignatureName, mapping.ProviderID, &mappingID)
		if err != nil {
			return fmt.Errorf("failed to check duplicate signature name: %w", err)
		}
		if exists {
			return fmt.Errorf("signature name '%s' with this provider already exists in this channel", req.SignatureName)
		}
		mapping.SignatureName = req.SignatureName
	}

	// 如果更新供应商签名，需要验证
	if req.ProviderSignatureID > 0 && req.ProviderSignatureID != mapping.ProviderSignatureID {
		providerSignature, err := s.providerSignatureDAO.GetByID(req.ProviderSignatureID)
		if err != nil {
			return fmt.Errorf("provider signature not found: %w", err)
		}
		// 验证供应商账号是否匹配
		if providerSignature.ProviderAccountID != mapping.ProviderID {
			return fmt.Errorf("provider signature does not belong to the current provider account")
		}
		mapping.ProviderSignatureID = req.ProviderSignatureID
	}

	if req.Status != nil {
		mapping.Status = *req.Status
	}

	return s.signatureMappingDAO.Update(mapping)
}

// DeleteChannelSignatureMapping 删除签名映射
func (s *AdminChannelService) DeleteChannelSignatureMapping(mappingID uint) error {
	// 检查映射是否存在
	_, err := s.signatureMappingDAO.GetByID(mappingID)
	if err != nil {
		return fmt.Errorf("signature mapping not found: %w", err)
	}

	// 删除映射
	return s.signatureMappingDAO.Delete(mappingID)
}

// GetAvailableProviderSignatures 获取可用的供应商签名列表（根据通道类型）
func (s *AdminChannelService) GetAvailableProviderSignatures(channelID uint) ([]*dto.ProviderSignatureResponse, error) {
	db := helper.GetHelper().GetDatabase()

	// 验证通道是否存在
	var channel model.Channel
	if err := db.First(&channel, channelID).Error; err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}

	// 获取所有匹配通道类型的供应商签名
	var signatures []*model.ProviderSignature
	if err := db.Joins("JOIN provider_accounts ON provider_accounts.id = provider_signatures.provider_account_id").
		Where("provider_accounts.provider_type = ? AND provider_accounts.status = 1 AND provider_signatures.status = 1", channel.Type).
		Preload("ProviderAccount").
		Find(&signatures).Error; err != nil {
		return nil, fmt.Errorf("failed to get provider signatures: %w", err)
	}

	items := make([]*dto.ProviderSignatureResponse, 0, len(signatures))
	for _, sig := range signatures {
		item := &dto.ProviderSignatureResponse{
			ID:            sig.ID,
			SignatureCode: sig.SignatureCode,
			SignatureName: sig.SignatureName,
			Status:        sig.Status,
			CreatedAt:     sig.CreatedAt.Format(time.RFC3339),
		}

		if sig.ProviderAccount != nil {
			item.ProviderAccountID = sig.ProviderAccountID
			item.ProviderAccountName = sig.ProviderAccount.AccountName
			item.ProviderCode = sig.ProviderAccount.ProviderCode
		}

		items = append(items, item)
	}

	return items, nil
}
