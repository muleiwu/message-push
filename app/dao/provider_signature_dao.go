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

	err := query.Order("created_at DESC").Find(&signatures).Error
	return signatures, err
}

// List 全局查询签名列表（带筛选和分页）
func (dao *ProviderSignatureDAO) List(providerAccountID *uint, status *int8, page, pageSize int) ([]*model.ProviderSignature, int64, error) {
	var signatures []*model.ProviderSignature
	var total int64

	query := dao.db.Model(&model.ProviderSignature{}).Preload("ProviderAccount")
	if providerAccountID != nil {
		query = query.Where("provider_account_id = ?", *providerAccountID)
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&signatures).Error; err != nil {
		return nil, 0, err
	}

	return signatures, total, nil
}

// GetByID 根据ID获取签名
func (dao *ProviderSignatureDAO) GetByID(id uint) (*model.ProviderSignature, error) {
	var signature model.ProviderSignature
	err := dao.db.Preload("ProviderAccount").First(&signature, id).Error
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
