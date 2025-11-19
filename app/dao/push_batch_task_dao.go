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
func (d *PushBatchTaskDAO) List(page, pageSize int) ([]*model.PushBatchTask, int64, error) {
	var batches []*model.PushBatchTask
	var total int64

	// 获取总数
	if err := d.db.Model(&model.PushBatchTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := d.db.Offset(offset).Limit(pageSize).
		Order("created_at DESC").
		Find(&batches).Error

	if err != nil {
		return nil, 0, err
	}

	return batches, total, nil
}
