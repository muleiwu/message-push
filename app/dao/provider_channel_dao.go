package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// ProviderChannelDAO 服务商通道DAO
type ProviderChannelDAO struct {
	db *gorm.DB
}

// NewProviderChannelDAO 创建DAO
func NewProviderChannelDAO() *ProviderChannelDAO {
	return &ProviderChannelDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建通道
func (d *ProviderChannelDAO) Create(channel *model.ProviderChannel) error {
	return d.db.Create(channel).Error
}

// GetByID 根据ID获取
func (d *ProviderChannelDAO) GetByID(id uint) (*model.ProviderChannel, error) {
	var channel model.ProviderChannel
	if err := d.db.First(&channel, id).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetByProviderID 根据服务商ID获取所有通道
func (d *ProviderChannelDAO) GetByProviderID(providerID uint) ([]*model.ProviderChannel, error) {
	var channels []*model.ProviderChannel
	err := d.db.Where("provider_id = ?", providerID).
		Order("id DESC").
		Find(&channels).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}

// GetByProviderIDAndType 根据服务商ID和消息类型获取通道
func (d *ProviderChannelDAO) GetByProviderIDAndType(providerID uint, messageType string) ([]*model.ProviderChannel, error) {
	var channels []*model.ProviderChannel
	err := d.db.Where("provider_id = ? AND message_type = ?", providerID, messageType).
		Order("id DESC").
		Find(&channels).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}

// List 获取列表
func (d *ProviderChannelDAO) List(page, pageSize int) ([]*model.ProviderChannel, int64, error) {
	var channels []*model.ProviderChannel
	var total int64

	// 获取总数
	if err := d.db.Model(&model.ProviderChannel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := d.db.Offset(offset).Limit(pageSize).
		Order("id DESC").
		Find(&channels).Error

	if err != nil {
		return nil, 0, err
	}

	return channels, total, nil
}

// Update 更新通道
func (d *ProviderChannelDAO) Update(channel *model.ProviderChannel) error {
	return d.db.Save(channel).Error
}

// UpdateStatus 更新状态
func (d *ProviderChannelDAO) UpdateStatus(id uint, status string) error {
	return d.db.Model(&model.ProviderChannel{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Delete 删除通道
func (d *ProviderChannelDAO) Delete(id uint) error {
	return d.db.Delete(&model.ProviderChannel{}, id).Error
}

// GetActiveChannels 获取启用的通道
func (d *ProviderChannelDAO) GetActiveChannels() ([]*model.ProviderChannel, error) {
	var channels []*model.ProviderChannel
	err := d.db.Where("status = ?", "active").
		Order("id DESC").
		Find(&channels).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}

// GetActiveChannelsByType 根据类型获取启用的通道
func (d *ProviderChannelDAO) GetActiveChannelsByType(messageType string) ([]*model.ProviderChannel, error) {
	var channels []*model.ProviderChannel
	err := d.db.Where("status = ? AND message_type = ?", "active", messageType).
		Order("id DESC").
		Find(&channels).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}
