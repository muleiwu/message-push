package dao

import (
	"strings"

	"cnb.cool/mliev/push/message-push/app/model"
	"gorm.io/gorm"
)

// ChannelSignatureMappingDAO 通道签名映射数据访问对象
type ChannelSignatureMappingDAO struct {
	db *gorm.DB
}

// NewChannelSignatureMappingDAO 创建签名映射DAO实例
func NewChannelSignatureMappingDAO(db *gorm.DB) *ChannelSignatureMappingDAO {
	return &ChannelSignatureMappingDAO{db: db}
}

// GetByChannelID 根据通道ID获取签名映射列表
func (dao *ChannelSignatureMappingDAO) GetByChannelID(channelID uint) ([]model.ChannelSignatureMapping, error) {
	var mappings []model.ChannelSignatureMapping
	err := dao.db.Preload("ProviderSignature").
		Preload("ProviderAccount").
		Where("channel_id = ?", channelID).
		Order("created_at DESC").
		Find(&mappings).Error
	return mappings, err
}

// GetByID 根据ID获取签名映射
func (dao *ChannelSignatureMappingDAO) GetByID(id uint) (*model.ChannelSignatureMapping, error) {
	var mapping model.ChannelSignatureMapping
	err := dao.db.Preload("ProviderSignature").
		Preload("ProviderAccount").
		First(&mapping, id).Error
	if err != nil {
		return nil, err
	}
	return &mapping, nil
}

// Create 创建签名映射
func (dao *ChannelSignatureMappingDAO) Create(mapping *model.ChannelSignatureMapping) error {
	return dao.db.Create(mapping).Error
}

// Update 更新签名映射
func (dao *ChannelSignatureMappingDAO) Update(mapping *model.ChannelSignatureMapping) error {
	return dao.db.Save(mapping).Error
}

// Delete 删除签名映射（软删除）
func (dao *ChannelSignatureMappingDAO) Delete(id uint) error {
	return dao.db.Delete(&model.ChannelSignatureMapping{}, id).Error
}

// CheckDuplicateSignatureName 检查同一通道下签名名称+供应商是否重复
func (dao *ChannelSignatureMappingDAO) CheckDuplicateSignatureName(channelID uint, signatureName string, providerID uint, excludeID *uint) (bool, error) {
	query := dao.db.Model(&model.ChannelSignatureMapping{}).
		Where("channel_id = ? AND signature_name = ? AND provider_id = ?", channelID, signatureName, providerID)

	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}

	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

// GetByChannelIDAndSignatureName 根据通道ID、签名名称和供应商ID获取供应商签名
func (dao *ChannelSignatureMappingDAO) GetByChannelIDAndSignatureName(channelID uint, signatureName string, providerID uint) (*model.ProviderSignature, error) {
	var mapping model.ChannelSignatureMapping
	err := dao.db.
		Preload("ProviderSignature").
		Joins("JOIN provider_signatures ON provider_signatures.id = channel_signature_mappings.provider_signature_id").
		Where("channel_signature_mappings.channel_id = ? AND channel_signature_mappings.signature_name = ? AND channel_signature_mappings.provider_id = ? AND channel_signature_mappings.status = 1", channelID, strings.TrimSpace(signatureName), providerID).
		Where("provider_signatures.provider_account_id = channel_signature_mappings.provider_id").
		Where("provider_signatures.status = 1 AND provider_signatures.deleted_at IS NULL").
		First(&mapping).Error
	if err != nil {
		return nil, err
	}
	if mapping.ProviderSignature == nil || strings.TrimSpace(mapping.ProviderSignature.SignatureCode) == "" {
		return nil, gorm.ErrRecordNotFound
	}
	return mapping.ProviderSignature, nil
}
