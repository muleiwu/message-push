package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// PushChannelDAO 推送通道数据访问对象
type PushChannelDAO struct {
	db *gorm.DB
}

// NewPushChannelDAO 创建PushChannelDAO
func NewPushChannelDAO() *PushChannelDAO {
	return &PushChannelDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建推送通道
func (d *PushChannelDAO) Create(channel *model.PushChannel) error {
	return d.db.Create(channel).Error
}

// GetByID 根据ID获取推送通道
func (d *PushChannelDAO) GetByID(id uint) (*model.PushChannel, error) {
	var channel model.PushChannel
	err := d.db.Where("id = ?", id).First(&channel).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

// GetByCode 根据通道代码获取推送通道
func (d *PushChannelDAO) GetByCode(channelCode string) (*model.PushChannel, error) {
	var channel model.PushChannel
	err := d.db.Where("channel_code = ?", channelCode).First(&channel).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

// Update 更新推送通道
func (d *PushChannelDAO) Update(channel *model.PushChannel) error {
	return d.db.Save(channel).Error
}

// Delete 删除推送通道
func (d *PushChannelDAO) Delete(id uint) error {
	return d.db.Delete(&model.PushChannel{}, id).Error
}

// List 获取推送通道列表
func (d *PushChannelDAO) List(page, pageSize int) ([]*model.PushChannel, int64, error) {
	var channels []*model.PushChannel
	var total int64

	offset := (page - 1) * pageSize

	if err := d.db.Model(&model.PushChannel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := d.db.Offset(offset).Limit(pageSize).Order("id DESC").Find(&channels).Error
	if err != nil {
		return nil, 0, err
	}

	return channels, total, nil
}

// GetByType 根据类型获取推送通道列表
func (d *PushChannelDAO) GetByType(channelType string) ([]*model.PushChannel, error) {
	var channels []*model.PushChannel
	err := d.db.Where("channel_type = ? AND status = 1", channelType).Find(&channels).Error
	if err != nil {
		return nil, err
	}
	return channels, nil
}
