package dao

import (
	"cnb.cool/mliev/push/message-push/app/dto"
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

// GetByID 根据ID获取日志
func (d *PushLogDAO) GetByID(id uint) (*model.PushLog, error) {
	var log model.PushLog
	if err := d.db.First(&log, id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

// List 获取日志列表（支持筛选）
func (d *PushLogDAO) List(req *dto.LogListRequest) ([]*model.PushLog, int64, error) {
	var logs []*model.PushLog
	var total int64

	query := d.db.Model(&model.PushLog{})

	// 筛选条件
	if req.TaskID != "" {
		query = query.Where("task_id = ?", req.TaskID)
	}
	if req.AppID != "" {
		query = query.Where("app_id = ?", req.AppID)
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.StartDate != "" {
		query = query.Where("created_at >= ?", req.StartDate+" 00:00:00")
	}
	if req.EndDate != "" {
		query = query.Where("created_at <= ?", req.EndDate+" 23:59:59")
	}
	// 注意：ProviderID 筛选需要关联查询，这里暂时只查 push_logs 表，如果需要关联查询可以扩展
	// 如果 push_logs 表里有冗余 provider_id 更好，目前只有 provider_channel_id

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (req.Page - 1) * req.PageSize
	err := query.Offset(offset).Limit(req.PageSize).
		Order("created_at DESC").
		Find(&logs).Error

	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
