package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// PushLogDAO 推送日志DAO
type PushLogDAO struct {
	db *gorm.DB
}

// NewPushLogDAO 创建DAO
func NewPushLogDAO() *PushLogDAO {
	return &PushLogDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建日志
func (d *PushLogDAO) Create(log *model.PushLog) error {
	return d.db.Create(log).Error
}

// GetByTaskID 根据任务ID获取日志
func (d *PushLogDAO) GetByTaskID(taskID string) ([]*model.PushLog, error) {
	var logs []*model.PushLog
	err := d.db.Where("task_id = ?", taskID).
		Order("created_at DESC").
		Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// List 获取日志列表
func (d *PushLogDAO) List(page, pageSize int) ([]*model.PushLog, int64, error) {
	var logs []*model.PushLog
	var total int64

	// 获取总数
	if err := d.db.Model(&model.PushLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := d.db.Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Find(&logs).Error

	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
