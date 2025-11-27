package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// CallbackLogDAO 回调日志数据访问对象
type CallbackLogDAO struct {
	db *gorm.DB
}

// NewCallbackLogDAO 创建 CallbackLogDAO
func NewCallbackLogDAO() *CallbackLogDAO {
	return &CallbackLogDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建回调日志
func (dao *CallbackLogDAO) Create(log *model.CallbackLog) error {
	return dao.db.Create(log).Error
}

// GetByTaskID 根据任务ID获取回调日志
func (dao *CallbackLogDAO) GetByTaskID(taskID string) ([]*model.CallbackLog, error) {
	var logs []*model.CallbackLog
	err := dao.db.Where("task_id = ?", taskID).
		Order("created_at DESC").
		Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByProviderID 根据服务商消息ID获取回调日志
func (dao *CallbackLogDAO) GetByProviderID(providerID string) ([]*model.CallbackLog, error) {
	var logs []*model.CallbackLog
	err := dao.db.Where("provider_id = ?", providerID).
		Order("created_at DESC").
		Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// List 获取回调日志列表（分页）
func (dao *CallbackLogDAO) List(appID string, page, pageSize int) ([]*model.CallbackLog, int64, error) {
	var logs []*model.CallbackLog
	var total int64

	query := dao.db.Model(&model.CallbackLog{})
	if appID != "" {
		query = query.Where("app_id = ?", appID)
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
