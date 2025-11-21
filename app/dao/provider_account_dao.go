package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// ProviderAccountDAO 服务商账号配置DAO
type ProviderAccountDAO struct {
	db *gorm.DB
}

// NewProviderAccountDAO 创建DAO
func NewProviderAccountDAO() *ProviderAccountDAO {
	return &ProviderAccountDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建服务商账号
func (d *ProviderAccountDAO) Create(account *model.ProviderAccount) error {
	return d.db.Create(account).Error
}

// GetByID 根据ID获取
func (d *ProviderAccountDAO) GetByID(id uint) (*model.ProviderAccount, error) {
	var account model.ProviderAccount
	if err := d.db.First(&account, id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// GetByAccountCode 根据账号编码获取
func (d *ProviderAccountDAO) GetByAccountCode(code string) (*model.ProviderAccount, error) {
	var account model.ProviderAccount
	err := d.db.Where("account_code = ?", code).First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// List 获取列表（带条件）
func (d *ProviderAccountDAO) List(page, pageSize int, providerType string, status int) ([]*model.ProviderAccount, int64, error) {
	var accounts []*model.ProviderAccount
	var total int64

	query := d.db.Model(&model.ProviderAccount{})

	// 添加过滤条件
	if providerType != "" {
		query = query.Where("provider_type = ?", providerType)
	}
	if status > 0 {
		query = query.Where("status = ?", status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).
		Order("id DESC").
		Find(&accounts).Error

	if err != nil {
		return nil, 0, err
	}

	return accounts, total, nil
}

// GetByProviderCode 根据服务商代码获取账号列表
func (d *ProviderAccountDAO) GetByProviderCode(providerCode string) ([]*model.ProviderAccount, error) {
	var accounts []*model.ProviderAccount
	err := d.db.Where("provider_code = ? AND status = ?", providerCode, 1).
		Order("id DESC").
		Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

// GetByType 根据消息类型获取账号列表
func (d *ProviderAccountDAO) GetByType(providerType string) ([]*model.ProviderAccount, error) {
	var accounts []*model.ProviderAccount
	err := d.db.Where("provider_type = ? AND status = ?", providerType, 1).
		Order("id DESC").
		Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

// GetActiveAccounts 获取启用的服务商账号
func (d *ProviderAccountDAO) GetActiveAccounts(providerType string) ([]*model.ProviderAccount, error) {
	var accounts []*model.ProviderAccount
	query := d.db.Where("status = ?", 1)

	if providerType != "" {
		query = query.Where("provider_type = ?", providerType)
	}

	err := query.Order("id DESC").Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

// Update 更新服务商账号
func (d *ProviderAccountDAO) Update(account *model.ProviderAccount) error {
	return d.db.Save(account).Error
}

// UpdateFields 更新指定字段
func (d *ProviderAccountDAO) UpdateFields(id uint, updates map[string]interface{}) error {
	return d.db.Model(&model.ProviderAccount{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateStatus 更新状态
func (d *ProviderAccountDAO) UpdateStatus(id uint, status int8) error {
	return d.db.Model(&model.ProviderAccount{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Delete 删除服务商账号（软删除）
func (d *ProviderAccountDAO) Delete(id uint) error {
	return d.db.Delete(&model.ProviderAccount{}, id).Error
}
