package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// TemplateBindingDAO 模板绑定数据访问对象
type TemplateBindingDAO struct {
	db *gorm.DB
}

// NewTemplateBindingDAO 创建模板绑定DAO
func NewTemplateBindingDAO() *TemplateBindingDAO {
	return &TemplateBindingDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建模板绑定
func (d *TemplateBindingDAO) Create(binding *model.TemplateBinding) error {
	return d.db.Create(binding).Error
}

// GetByID 根据ID获取模板绑定
func (d *TemplateBindingDAO) GetByID(id uint) (*model.TemplateBinding, error) {
	var binding model.TemplateBinding
	err := d.db.Preload("MessageTemplate").
		Preload("ProviderTemplate").
		Preload("ProviderAccount").
		Where("id = ?", id).First(&binding).Error
	return &binding, err
}

// GetByMessageTemplateID 根据系统模板ID获取所有模板绑定
func (d *TemplateBindingDAO) GetByMessageTemplateID(messageTemplateID uint) ([]*model.TemplateBinding, error) {
	var bindings []*model.TemplateBinding
	err := d.db.Preload("MessageTemplate").
		Preload("ProviderTemplate").
		Preload("ProviderTemplate.ProviderAccount").
		Preload("ProviderAccount").
		Where("message_template_id = ? AND status = 1", messageTemplateID).
		Order("priority ASC").
		Find(&bindings).Error
	return bindings, err
}

// Update 更新模板绑定
func (d *TemplateBindingDAO) Update(binding *model.TemplateBinding) error {
	return d.db.Save(binding).Error
}

// Delete 删除模板绑定（软删除）
func (d *TemplateBindingDAO) Delete(id uint) error {
	return d.db.Delete(&model.TemplateBinding{}, id).Error
}

// List 查询模板绑定列表
func (d *TemplateBindingDAO) List(messageTemplateID, providerID *uint, status *int8, page, pageSize int) ([]*model.TemplateBinding, int64, error) {
	var bindings []*model.TemplateBinding
	var total int64

	query := d.db.Model(&model.TemplateBinding{}).
		Preload("MessageTemplate").
		Preload("ProviderTemplate").
		Preload("ProviderAccount")

	if messageTemplateID != nil {
		query = query.Where("message_template_id = ?", *messageTemplateID)
	}
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
	err := query.Order("priority ASC, id DESC").Offset(offset).Limit(pageSize).Find(&bindings).Error

	return bindings, total, err
}

// GetByMessageTemplateAndProvider 根据系统模板和供应商获取绑定（按优先级排序）
func (d *TemplateBindingDAO) GetByMessageTemplateAndProvider(messageTemplateID, providerID uint) ([]*model.TemplateBinding, error) {
	var bindings []*model.TemplateBinding
	err := d.db.Preload("ProviderTemplate").
		Where("message_template_id = ? AND provider_id = ? AND status = 1", messageTemplateID, providerID).
		Order("priority ASC").
		Find(&bindings).Error
	return bindings, err
}

// GetActiveBindingByTemplateCodeAndProvider 根据系统模板代码和供应商获取启用的绑定关系
func (d *TemplateBindingDAO) GetActiveBindingByTemplateCodeAndProvider(templateCode string, providerID uint) (*model.TemplateBinding, error) {
	var binding model.TemplateBinding
	err := d.db.Joins("JOIN message_templates ON message_templates.id = template_bindings.message_template_id").
		Preload("MessageTemplate").
		Preload("ProviderTemplate").
		Where("message_templates.template_code = ? AND template_bindings.provider_id = ? AND template_bindings.status = 1 AND message_templates.status = 1", templateCode, providerID).
		Order("template_bindings.priority ASC").
		First(&binding).Error
	return &binding, err
}

// ExistsByTemplates 检查模板绑定是否已存在
func (d *TemplateBindingDAO) ExistsByTemplates(messageTemplateID, providerTemplateID uint, excludeID uint) (bool, error) {
	var count int64
	query := d.db.Model(&model.TemplateBinding{}).Where("message_template_id = ? AND provider_template_id = ?", messageTemplateID, providerTemplateID)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}
