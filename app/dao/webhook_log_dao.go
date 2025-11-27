package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// WebhookLogDAO Webhook日志数据访问对象
type WebhookLogDAO struct {
	db *gorm.DB
}

// NewWebhookLogDAO 创建 WebhookLogDAO
func NewWebhookLogDAO() *WebhookLogDAO {
	return &WebhookLogDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建Webhook日志
func (dao *WebhookLogDAO) Create(log *model.WebhookLog) error {
	return dao.db.Create(log).Error
}

// GetByTaskID 根据任务ID获取Webhook日志
func (dao *WebhookLogDAO) GetByTaskID(taskID string) ([]*model.WebhookLog, error) {
	var logs []*model.WebhookLog
	err := dao.db.Where("task_id = ?", taskID).
		Order("created_at DESC").
		Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByWebhookConfigID 根据Webhook配置ID获取日志
func (dao *WebhookLogDAO) GetByWebhookConfigID(configID uint) ([]*model.WebhookLog, error) {
	var logs []*model.WebhookLog
	err := dao.db.Where("webhook_config_id = ?", configID).
		Order("created_at DESC").
		Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// List 获取Webhook日志列表（分页）
func (dao *WebhookLogDAO) List(appID string, status string, page, pageSize int) ([]*model.WebhookLog, int64, error) {
	var logs []*model.WebhookLog
	var total int64

	query := dao.db.Model(&model.WebhookLog{})
	if appID != "" {
		query = query.Where("app_id = ?", appID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Find(&logs).Error

	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
