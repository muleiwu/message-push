package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"gorm.io/gorm"
)

// ProviderSignatureDAO 供应商签名数据访问对象
type ProviderSignatureDAO struct {
	db *gorm.DB
}

// NewProviderSignatureDAO 创建签名DAO实例
func NewProviderSignatureDAO(db *gorm.DB) *ProviderSignatureDAO {
	return &ProviderSignatureDAO{db: db}
}

// GetByProviderAccountID 根据供应商账号ID获取签名列表
func (dao *ProviderSignatureDAO) GetByProviderAccountID(providerAccountID uint, status *int8) ([]model.ProviderSignature, error) {
	var signatures []model.ProviderSignature
	query := dao.db.Where("provider_account_id = ?", providerAccountID)

	if status != nil {
		query = query.Where("status = ?", *status)
	}

	err := query.Order("is_default DESC, created_at DESC").Find(&signatures).Error
	return signatures, err
}

// GetDefaultSignature 获取指定账号的默认签名
func (dao *ProviderSignatureDAO) GetDefaultSignature(providerAccountID uint) (*model.ProviderSignature, error) {
	var signature model.ProviderSignature
	err := dao.db.Where("provider_account_id = ? AND is_default = 1 AND status = 1", providerAccountID).
		First(&signature).Error

	if err != nil {
		return nil, err
	}
	return &signature, nil
}

// GetByID 根据ID获取签名
func (dao *ProviderSignatureDAO) GetByID(id uint) (*model.ProviderSignature, error) {
	var signature model.ProviderSignature
	err := dao.db.First(&signature, id).Error
	if err != nil {
		return nil, err
	}
	return &signature, nil
}

// Create 创建签名
func (dao *ProviderSignatureDAO) Create(signature *model.ProviderSignature) error {
	return dao.db.Create(signature).Error
}

// Update 更新签名
func (dao *ProviderSignatureDAO) Update(signature *model.ProviderSignature) error {
	return dao.db.Save(signature).Error
}

// Delete 删除签名（软删除）
func (dao *ProviderSignatureDAO) Delete(id uint) error {
	return dao.db.Delete(&model.ProviderSignature{}, id).Error
}

// UpdateDefaultStatus 更新默认签名状态（设置新默认签名时，取消同一账号下其他签名的默认状态）
func (dao *ProviderSignatureDAO) UpdateDefaultStatus(providerAccountID uint, newDefaultID uint) error {
	return dao.db.Transaction(func(tx *gorm.DB) error {
		// 先将该账号下所有签名的默认状态设为0
		if err := tx.Model(&model.ProviderSignature{}).
			Where("provider_account_id = ?", providerAccountID).
			Update("is_default", 0).Error; err != nil {
			return err
		}

		// 再将指定签名设为默认
		if err := tx.Model(&model.ProviderSignature{}).
			Where("id = ?", newDefaultID).
			Update("is_default", 1).Error; err != nil {
			return err
		}

		return nil
	})
}

// CheckSignatureExists 检查签名代码是否已存在（同一账号下）
func (dao *ProviderSignatureDAO) CheckSignatureExists(providerAccountID uint, signatureCode string, excludeID *uint) (bool, error) {
	query := dao.db.Model(&model.ProviderSignature{}).
		Where("provider_account_id = ? AND signature_code = ?", providerAccountID, signatureCode)

	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}

	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}
