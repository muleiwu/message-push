package dao

import (
	"fmt"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProviderTemplateDAO 供应商模板数据访问对象
type ProviderTemplateDAO struct {
	db *gorm.DB
}

// NewProviderTemplateDAO 创建供应商模板DAO
func NewProviderTemplateDAO() *ProviderTemplateDAO {
	return &ProviderTemplateDAO{
		db: helper.GetDatabase(),
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
	return d.db.Transaction(func(tx *gorm.DB) error {
		current, err := LockProviderTemplate(tx, template.ID)
		if err != nil {
			return err
		}
		if current.DeletedAt.Valid {
			return gorm.ErrRecordNotFound
		}
		if current.SyncedAt != nil && (current.TemplateContent != template.TemplateContent || current.ContentType != template.ContentType) {
			return fmt.Errorf("已同步模板的正文请通过供应商操作修改")
		}
		var account model.ProviderAccount
		if err := tx.First(&account, current.ProviderID).Error; err != nil {
			return err
		}
		if account.ProviderType == "sms" {
			if err := SetTemplateContent(tx, current, template.TemplateContent); err != nil {
				return err
			}
			template.Variables = "[]"
		}
		template.ContentVersion = current.ContentVersion
		return tx.Model(current).Updates(map[string]any{
			"template_name": template.TemplateName, "content_type": template.ContentType,
			"template_content": template.TemplateContent, "variables": template.Variables,
			"status": template.Status, "remark": template.Remark, "content_version": current.ContentVersion,
		}).Error
	})
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
	err := d.db.Preload("ProviderAccount").Where("provider_id = ? AND status = 1", providerID).Where(model.ResourceUsableSQL("provider_templates")).Find(&templates).Error
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

// LockProviderTemplate serializes content changes with mapping confirmation.
func LockProviderTemplate(tx *gorm.DB, id uint) (*model.ProviderTemplate, error) {
	var t model.ProviderTemplate
	err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).First(&t, id).Error
	return &t, err
}

func ResetTemplateBindings(tx *gorm.DB, id uint) error {
	return tx.Unscoped().Model(&model.ChannelTemplateBinding{}).Where("provider_template_id = ?", id).
		Updates(map[string]any{"param_mapping": "[]", "mapped_content_version": 0}).Error
}

// SetTemplateContent must run in the same transaction as saving the template.
func SetTemplateContent(tx *gorm.DB, t *model.ProviderTemplate, content string) error {
	if t.ID != 0 && t.TemplateContent != content {
		if err := ResetTemplateBindings(tx, t.ID); err != nil {
			return err
		}
		t.ContentVersion++
	}
	if t.ContentVersion == 0 {
		t.ContentVersion = 1
	}
	t.TemplateContent = content
	return nil
}

func (d *ProviderTemplateDAO) BindingChannelIDs(id uint) ([]uint, error) {
	var ids []uint
	err := d.db.Unscoped().Model(&model.ChannelTemplateBinding{}).Where("provider_template_id = ?", id).Distinct("channel_id").Pluck("channel_id", &ids).Error
	return ids, err
}
