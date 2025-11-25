package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// WebhookConfigDAO Webhook 配置数据访问对象
type WebhookConfigDAO struct {
	db *gorm.DB
}

// NewWebhookConfigDAO 创建 WebhookConfigDAO
func NewWebhookConfigDAO() *WebhookConfigDAO {
	return &WebhookConfigDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建 Webhook 配置
func (dao *WebhookConfigDAO) Create(config *model.WebhookConfig) error {
	return dao.db.Create(config).Error
}

// Update 更新 Webhook 配置
func (dao *WebhookConfigDAO) Update(config *model.WebhookConfig) error {
	return dao.db.Save(config).Error
}

// Delete 删除 Webhook 配置
func (dao *WebhookConfigDAO) Delete(id uint) error {
	return dao.db.Delete(&model.WebhookConfig{}, id).Error
}

// GetByID 根据 ID 获取 Webhook 配置
func (dao *WebhookConfigDAO) GetByID(id uint) (*model.WebhookConfig, error) {
	var config model.WebhookConfig
	err := dao.db.First(&config, id).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetByAppID 根据应用 ID 获取 Webhook 配置
func (dao *WebhookConfigDAO) GetByAppID(appID string) (*model.WebhookConfig, error) {
	var config model.WebhookConfig
	err := dao.db.Where("app_id = ?", appID).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// GetEnabledByAppID 根据应用 ID 获取已启用的 Webhook 配置
func (dao *WebhookConfigDAO) GetEnabledByAppID(appID string) (*model.WebhookConfig, error) {
	var config model.WebhookConfig
	err := dao.db.Where("app_id = ? AND status = 1", appID).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// List 获取所有 Webhook 配置
func (dao *WebhookConfigDAO) List() ([]*model.WebhookConfig, error) {
	var configs []*model.WebhookConfig
	err := dao.db.Find(&configs).Error
	return configs, err
}

// ListEnabled 获取所有已启用的 Webhook 配置
func (dao *WebhookConfigDAO) ListEnabled() ([]*model.WebhookConfig, error) {
	var configs []*model.WebhookConfig
	err := dao.db.Where("status = 1").Find(&configs).Error
	return configs, err
}
