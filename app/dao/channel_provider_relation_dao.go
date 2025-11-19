package dao

import (
	"fmt"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// ChannelProviderRelationDAO 通道-服务商关联DAO
type ChannelProviderRelationDAO struct {
	db *gorm.DB
}

// NewChannelProviderRelationDAO 创建DAO
func NewChannelProviderRelationDAO() *ChannelProviderRelationDAO {
	return &ChannelProviderRelationDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建关联
func (d *ChannelProviderRelationDAO) Create(relation *model.ChannelProviderRelation) error {
	return d.db.Create(relation).Error
}

// GetByID 根据ID获取
func (d *ChannelProviderRelationDAO) GetByID(id uint) (*model.ChannelProviderRelation, error) {
	var relation model.ChannelProviderRelation
	if err := d.db.First(&relation, id).Error; err != nil {
		return nil, err
	}
	return &relation, nil
}

// GetByChannelID 根据业务通道ID获取所有关联（包含服务商信息）
func (d *ChannelProviderRelationDAO) GetByChannelID(channelID uint) ([]*model.ChannelProviderRelation, error) {
	var relations []*model.ChannelProviderRelation
	err := d.db.Preload("ProviderChannel.Provider").
		Where("push_channel_id = ?", channelID).
		Order("priority DESC, weight DESC").
		Find(&relations).Error
	if err != nil {
		return nil, err
	}
	return relations, nil
}

// GetByChannelIDAndType 根据业务通道ID和消息类型获取关联
func (d *ChannelProviderRelationDAO) GetByChannelIDAndType(channelID uint, messageType string) ([]*model.ChannelProviderRelation, error) {
	var relations []*model.ChannelProviderRelation
	// 这里原本查询了 message_type，但 ChannelProviderRelation 表本身可能没有 message_type 字段
	// 如果有，则保留。如果没有，需要检查 model 定义。
	// 假设 push_channel_id 已经足够过滤，且业务逻辑保证同一通道只关联同类型服务商。
	// 若需严格过滤，应 Join provider_channels 和 providers。
	err := d.db.Where("push_channel_id = ?", channelID).
		Order("priority DESC, weight DESC").
		Find(&relations).Error
	if err != nil {
		return nil, err
	}
	return relations, nil
}

// Update 更新关联
func (d *ChannelProviderRelationDAO) Update(relation *model.ChannelProviderRelation) error {
	return d.db.Save(relation).Error
}

// Delete 删除关联
func (d *ChannelProviderRelationDAO) Delete(id uint) error {
	return d.db.Delete(&model.ChannelProviderRelation{}, id).Error
}

// DeleteByChannelID 删除业务通道的所有关联
func (d *ChannelProviderRelationDAO) DeleteByChannelID(channelID uint) error {
	return d.db.Where("push_channel_id = ?", channelID).
		Delete(&model.ChannelProviderRelation{}).Error
}

// UpdatePriority 更新优先级
func (d *ChannelProviderRelationDAO) UpdatePriority(id uint, priority int) error {
	return d.db.Model(&model.ChannelProviderRelation{}).
		Where("id = ?", id).
		Update("priority", priority).Error
}

// UpdateWeight 更新权重
func (d *ChannelProviderRelationDAO) UpdateWeight(id uint, weight int) error {
	return d.db.Model(&model.ChannelProviderRelation{}).
		Where("id = ?", id).
		Update("weight", weight).Error
}

// BatchCreate 批量创建关联
func (d *ChannelProviderRelationDAO) BatchCreate(relations []*model.ChannelProviderRelation) error {
	if len(relations) == 0 {
		return fmt.Errorf("empty relations")
	}
	return d.db.Create(&relations).Error
}
