package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// ProviderTemplateDAO 供应商模板数据访问对象
type ProviderTemplateDAO struct {
	db *gorm.DB
}

// NewProviderTemplateDAO 创建供应商模板DAO
func NewProviderTemplateDAO() *ProviderTemplateDAO {
	return &ProviderTemplateDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建供应商模板
func (d *ProviderTemplateDAO) Create(template *model.ProviderTemplate) error {
	return d.db.Create(template).Error
}

// GetByID 根据ID获取供应商模板
func (d *ProviderTemplateDAO) GetByID(id uint) (*model.ProviderTemplate, error) {
	var template model.ProviderTemplate
	err := d.db.Preload("ProviderAccount").Where("id = ?", id).First(&template).Error
	return &template, err
}

// GetByProviderAndCode 根据供应商ID和模板代码获取供应商模板
func (d *ProviderTemplateDAO) GetByProviderAndCode(providerID uint, templateCode string) (*model.ProviderTemplate, error) {
	var template model.ProviderTemplate
	err := d.db.Where("provider_id = ? AND template_code = ?", providerID, templateCode).First(&template).Error
	return &template, err
}

// Update 更新供应商模板
func (d *ProviderTemplateDAO) Update(template *model.ProviderTemplate) error {
	return d.db.Save(template).Error
}

// Delete 删除供应商模板（软删除）
func (d *ProviderTemplateDAO) Delete(id uint) error {
	return d.db.Delete(&model.ProviderTemplate{}, id).Error
}

// List 查询供应商模板列表
func (d *ProviderTemplateDAO) List(providerID *uint, status *int8, page, pageSize int) ([]*model.ProviderTemplate, int64, error) {
	var templates []*model.ProviderTemplate
	var total int64

	query := d.db.Model(&model.ProviderTemplate{}).Preload("ProviderAccount")

	if providerID != nil {
		query = query.Where("provider_id = ?", *providerID)
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&templates).Error

	return templates, total, err
}

// GetActiveByProvider 获取供应商的所有启用模板
func (d *ProviderTemplateDAO) GetActiveByProvider(providerID uint) ([]*model.ProviderTemplate, error) {
	var templates []*model.ProviderTemplate
	err := d.db.Where("provider_id = ? AND status = 1", providerID).Find(&templates).Error
	return templates, err
}

// ExistsByProviderAndCode 检查供应商模板代码是否已存在
func (d *ProviderTemplateDAO) ExistsByProviderAndCode(providerID uint, templateCode string, excludeID uint) (bool, error) {
	var count int64
	query := d.db.Model(&model.ProviderTemplate{}).Where("provider_id = ? AND template_code = ?", providerID, templateCode)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}
