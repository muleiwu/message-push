package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// ChannelTemplateBindingDAO 通道模板绑定配置 DAO
type ChannelTemplateBindingDAO struct{}

// NewChannelTemplateBindingDAO 创建 ChannelTemplateBindingDAO 实例
func NewChannelTemplateBindingDAO() *ChannelTemplateBindingDAO {
	return &ChannelTemplateBindingDAO{}
}

// Create 创建绑定配置
func (d *ChannelTemplateBindingDAO) Create(binding *model.ChannelTemplateBinding) error {
	db := helper.GetHelper().GetDatabase()
	return db.Create(binding).Error
}

// BatchCreate 批量创建绑定配置
func (d *ChannelTemplateBindingDAO) BatchCreate(bindings []*model.ChannelTemplateBinding) error {
	if len(bindings) == 0 {
		return nil
	}
	db := helper.GetHelper().GetDatabase()
	return db.Create(&bindings).Error
}

// GetByID 根据ID查询
func (d *ChannelTemplateBindingDAO) GetByID(id uint) (*model.ChannelTemplateBinding, error) {
	var binding model.ChannelTemplateBinding
	db := helper.GetHelper().GetDatabase()
	err := db.Preload("TemplateBinding").
		Preload("TemplateBinding.ProviderTemplate").
		Preload("TemplateBinding.ProviderTemplate.ProviderAccount").
		Preload("TemplateBinding.MessageTemplate").
		First(&binding, id).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

// GetByChannelID 根据通道ID查询所有绑定配置
func (d *ChannelTemplateBindingDAO) GetByChannelID(channelID uint) ([]*model.ChannelTemplateBinding, error) {
	var bindings []*model.ChannelTemplateBinding
	db := helper.GetHelper().GetDatabase()
	err := db.Where("channel_id = ?", channelID).
		Preload("TemplateBinding").
		Preload("TemplateBinding.ProviderTemplate").
		Preload("TemplateBinding.ProviderTemplate.ProviderAccount").
		Preload("TemplateBinding.MessageTemplate").
		Order("priority ASC, weight DESC").
		Find(&bindings).Error
	if err != nil {
		return nil, err
	}
	return bindings, nil
}

// GetActiveByChannelID 根据通道ID查询所有激活的绑定配置
func (d *ChannelTemplateBindingDAO) GetActiveByChannelID(channelID uint) ([]*model.ChannelTemplateBinding, error) {
	var bindings []*model.ChannelTemplateBinding
	db := helper.GetHelper().GetDatabase()
	err := db.Where("channel_id = ? AND is_active = 1", channelID).
		Preload("TemplateBinding").
		Preload("TemplateBinding.ProviderTemplate").
		Preload("TemplateBinding.ProviderTemplate.ProviderAccount").
		Preload("TemplateBinding.MessageTemplate").
		Order("priority ASC, weight DESC").
		Find(&bindings).Error
	if err != nil {
		return nil, err
	}
	return bindings, nil
}

// Update 更新配置
func (d *ChannelTemplateBindingDAO) Update(id uint, updates map[string]interface{}) error {
	db := helper.GetHelper().GetDatabase()
	return db.Model(&model.ChannelTemplateBinding{}).Where("id = ?", id).Updates(updates).Error
}

// UpdateWeight 更新权重
func (d *ChannelTemplateBindingDAO) UpdateWeight(id uint, weight int) error {
	db := helper.GetHelper().GetDatabase()
	return db.Model(&model.ChannelTemplateBinding{}).Where("id = ?", id).Update("weight", weight).Error
}

// UpdatePriority 更新优先级
func (d *ChannelTemplateBindingDAO) UpdatePriority(id uint, priority int) error {
	db := helper.GetHelper().GetDatabase()
	return db.Model(&model.ChannelTemplateBinding{}).Where("id = ?", id).Update("priority", priority).Error
}

// UpdateStatus 更新状态
func (d *ChannelTemplateBindingDAO) UpdateStatus(id uint, status int8) error {
	db := helper.GetHelper().GetDatabase()
	return db.Model(&model.ChannelTemplateBinding{}).Where("id = ?", id).Update("is_active", status).Error
}

// Delete 删除配置
func (d *ChannelTemplateBindingDAO) Delete(id uint) error {
	db := helper.GetHelper().GetDatabase()
	return db.Delete(&model.ChannelTemplateBinding{}, id).Error
}

// DeleteByChannelID 删除通道的所有绑定配置
func (d *ChannelTemplateBindingDAO) DeleteByChannelID(channelID uint) error {
	db := helper.GetHelper().GetDatabase()
	return db.Where("channel_id = ?", channelID).Delete(&model.ChannelTemplateBinding{}).Error
}
