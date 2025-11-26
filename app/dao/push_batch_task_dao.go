package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// PushBatchTaskDAO 批量任务DAO
type PushBatchTaskDAO struct {
	db *gorm.DB
}

// NewPushBatchTaskDAO 创建DAO
func NewPushBatchTaskDAO() *PushBatchTaskDAO {
	return &PushBatchTaskDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建批量任务
func (d *PushBatchTaskDAO) Create(batch *model.PushBatchTask) error {
	return d.db.Create(batch).Error
}

// GetByBatchID 根据批次ID获取
func (d *PushBatchTaskDAO) GetByBatchID(batchID string) (*model.PushBatchTask, error) {
	var batch model.PushBatchTask
	err := d.db.Where("batch_id = ?", batchID).First(&batch).Error
	if err != nil {
		return nil, err
	}
	return &batch, nil
}

// Update 更新批量任务
func (d *PushBatchTaskDAO) Update(batch *model.PushBatchTask) error {
	return d.db.Save(batch).Error
}

// IncrementSuccess 增加成功计数
func (d *PushBatchTaskDAO) IncrementSuccess(batchID string) error {
	return d.db.Model(&model.PushBatchTask{}).
		Where("batch_id = ?", batchID).
		UpdateColumn("success_count", gorm.Expr("success_count + ?", 1)).Error
}

// IncrementFailed 增加失败计数
func (d *PushBatchTaskDAO) IncrementFailed(batchID string) error {
	return d.db.Model(&model.PushBatchTask{}).
		Where("batch_id = ?", batchID).
		UpdateColumn("failed_count", gorm.Expr("failed_count + ?", 1)).Error
}

// List 获取批量任务列表
func (d *PushBatchTaskDAO) List(page, pageSize int, filters map[string]interface{}) ([]*model.PushBatchTask, int64, error) {
	var batches []*model.PushBatchTask
	var total int64

	offset := (page - 1) * pageSize
	query := d.db.Model(&model.PushBatchTask{})

	// 应用过滤条件
	if appID, ok := filters["app_id"]; ok {
		query = query.Where("app_id = ?", appID)
	}
	if status, ok := filters["status"]; ok {
		query = query.Where("status = ?", status)
	}
	if batchID, ok := filters["batch_id"]; ok {
		query = query.Where("batch_id LIKE ?", "%"+batchID.(string)+"%")
	}
	if startDate, ok := filters["start_date"]; ok {
		query = query.Where("DATE(created_at) >= ?", startDate)
	}
	if endDate, ok := filters["end_date"]; ok {
		query = query.Where("DATE(created_at) <= ?", endDate)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	err := query.Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Find(&batches).Error

	if err != nil {
		return nil, 0, err
	}

	return batches, total, nil
}

// GetByID 根据ID获取批量任务
func (d *PushBatchTaskDAO) GetByID(id uint) (*model.PushBatchTask, error) {
	var batch model.PushBatchTask
	err := d.db.Where("id = ?", id).First(&batch).Error
	if err != nil {
		return nil, err
	}
	return &batch, nil
}
