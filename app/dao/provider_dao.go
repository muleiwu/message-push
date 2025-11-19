package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// ProviderDAO 服务商DAO
type ProviderDAO struct {
	db *gorm.DB
}

// NewProviderDAO 创建DAO
func NewProviderDAO() *ProviderDAO {
	return &ProviderDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建服务商
func (d *ProviderDAO) Create(provider *model.Provider) error {
	return d.db.Create(provider).Error
}

// GetByID 根据ID获取
func (d *ProviderDAO) GetByID(id uint) (*model.Provider, error) {
	var provider model.Provider
	if err := d.db.First(&provider, id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

// GetByCode 根据编码获取
func (d *ProviderDAO) GetByCode(code string) (*model.Provider, error) {
	var provider model.Provider
	err := d.db.Where("code = ?", code).First(&provider).Error
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

// List 获取列表
func (d *ProviderDAO) List(page, pageSize int) ([]*model.Provider, int64, error) {
	var providers []*model.Provider
	var total int64

	// 获取总数
	if err := d.db.Model(&model.Provider{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := d.db.Offset(offset).Limit(pageSize).
		Order("id DESC").
		Find(&providers).Error

	if err != nil {
		return nil, 0, err
	}

	return providers, total, nil
}

// GetByType 根据类型获取
func (d *ProviderDAO) GetByType(providerType string) ([]*model.Provider, error) {
	var providers []*model.Provider
	err := d.db.Where("type = ?", providerType).
		Order("id DESC").
		Find(&providers).Error
	if err != nil {
		return nil, err
	}
	return providers, nil
}

// GetActiveProviders 获取启用的服务商
func (d *ProviderDAO) GetActiveProviders() ([]*model.Provider, error) {
	var providers []*model.Provider
	err := d.db.Where("status = ?", "active").
		Order("id DESC").
		Find(&providers).Error
	if err != nil {
		return nil, err
	}
	return providers, nil
}

// Update 更新服务商
func (d *ProviderDAO) Update(provider *model.Provider) error {
	return d.db.Save(provider).Error
}

// UpdateStatus 更新状态
func (d *ProviderDAO) UpdateStatus(id uint, status string) error {
	return d.db.Model(&model.Provider{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// Delete 删除服务商
func (d *ProviderDAO) Delete(id uint) error {
	return d.db.Delete(&model.Provider{}, id).Error
}
