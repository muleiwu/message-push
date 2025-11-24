package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// MessageTemplateDAO 系统模板数据访问对象
type MessageTemplateDAO struct {
	db *gorm.DB
}

// NewMessageTemplateDAO 创建系统模板DAO
func NewMessageTemplateDAO() *MessageTemplateDAO {
	return &MessageTemplateDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建系统模板
func (d *MessageTemplateDAO) Create(template *model.MessageTemplate) error {
	return d.db.Create(template).Error
}

// GetByID 根据ID获取系统模板
func (d *MessageTemplateDAO) GetByID(id uint) (*model.MessageTemplate, error) {
	var template model.MessageTemplate
	err := d.db.Where("id = ?", id).First(&template).Error
	return &template, err
}

// GetByCode 根据模板代码获取系统模板
func (d *MessageTemplateDAO) GetByCode(code string) (*model.MessageTemplate, error) {
	var template model.MessageTemplate
	err := d.db.Where("template_code = ?", code).First(&template).Error
	return &template, err
}

// Update 更新系统模板
func (d *MessageTemplateDAO) Update(template *model.MessageTemplate) error {
	return d.db.Save(template).Error
}

// Delete 删除系统模板（软删除）
func (d *MessageTemplateDAO) Delete(id uint) error {
	return d.db.Delete(&model.MessageTemplate{}, id).Error
}

// List 查询系统模板列表
func (d *MessageTemplateDAO) List(messageType string, status *int8, page, pageSize int) ([]*model.MessageTemplate, int64, error) {
	var templates []*model.MessageTemplate
	var total int64

	query := d.db.Model(&model.MessageTemplate{})

	if messageType != "" {
		query = query.Where("message_type = ?", messageType)
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

// GetActiveByTypeAndCode 根据消息类型和模板代码获取启用的系统模板
func (d *MessageTemplateDAO) GetActiveByTypeAndCode(messageType, templateCode string) (*model.MessageTemplate, error) {
	var template model.MessageTemplate
	err := d.db.Where("message_type = ? AND template_code = ? AND status = 1", messageType, templateCode).First(&template).Error
	return &template, err
}

// ExistsByCode 检查模板代码是否已存在
func (d *MessageTemplateDAO) ExistsByCode(code string, excludeID uint) (bool, error) {
	var count int64
	query := d.db.Model(&model.MessageTemplate{}).Where("template_code = ?", code)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}
