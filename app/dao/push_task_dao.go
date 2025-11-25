package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// PushTaskDAO 推送任务数据访问对象
type PushTaskDAO struct {
	db *gorm.DB
}

// NewPushTaskDAO 创建PushTaskDAO
func NewPushTaskDAO() *PushTaskDAO {
	return &PushTaskDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建任务
func (d *PushTaskDAO) Create(task *model.PushTask) error {
	return d.db.Create(task).Error
}

// GetByID 根据ID获取任务
func (d *PushTaskDAO) GetByID(id uint) (*model.PushTask, error) {
	var task model.PushTask
	err := d.db.Where("id = ?", id).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// GetByTaskID 根据TaskID获取任务
func (d *PushTaskDAO) GetByTaskID(taskID string) (*model.PushTask, error) {
	var task model.PushTask
	err := d.db.Where("task_id = ?", taskID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// Update 更新任务
func (d *PushTaskDAO) Update(task *model.PushTask) error {
	return d.db.Save(task).Error
}

// UpdateStatus 更新任务状态
func (d *PushTaskDAO) UpdateStatus(taskID, status string) error {
	return d.db.Model(&model.PushTask{}).
		Where("task_id = ?", taskID).
		Update("status", status).Error
}

// GetByAppIDAndStatus 根据应用ID和状态获取任务列表
func (d *PushTaskDAO) GetByAppIDAndStatus(appID, status string, limit int) ([]*model.PushTask, error) {
	var tasks []*model.PushTask
	err := d.db.Where("app_id = ? AND status = ?", appID, status).
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetPendingTasks 获取待处理的任务
func (d *PushTaskDAO) GetPendingTasks(limit int) ([]*model.PushTask, error) {
	var tasks []*model.PushTask
	err := d.db.Where("status = ?", "pending").
		Order("created_at ASC").
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// GetScheduledTasks 获取到期的定时任务
func (d *PushTaskDAO) GetScheduledTasks(limit int) ([]*model.PushTask, error) {
	var tasks []*model.PushTask
	err := d.db.Where("status = ? AND scheduled_at <= NOW()", "pending").
		Order("scheduled_at ASC").
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// IncrementRetryCount 增加重试次数
func (d *PushTaskDAO) IncrementRetryCount(taskID string) error {
	return d.db.Model(&model.PushTask{}).
		Where("task_id = ?", taskID).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

// GetByProviderID 根据服务商消息ID获取任务
func (d *PushTaskDAO) GetByProviderID(providerMsgID string) (*model.PushTask, error) {
	var task model.PushTask
	err := d.db.Where("provider_msg_id = ?", providerMsgID).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateProviderMsgID 更新服务商消息ID
func (d *PushTaskDAO) UpdateProviderMsgID(taskID, providerMsgID string) error {
	return d.db.Model(&model.PushTask{}).
		Where("task_id = ?", taskID).
		Update("provider_msg_id", providerMsgID).Error
}

// List 获取任务列表（分页）
func (d *PushTaskDAO) List(page, pageSize int, filters map[string]interface{}) ([]*model.PushTask, int64, error) {
	var tasks []*model.PushTask
	var total int64

	offset := (page - 1) * pageSize
	query := d.db.Model(&model.PushTask{})

	// 应用过滤条件
	if appID, ok := filters["app_id"]; ok {
		query = query.Where("app_id = ?", appID)
	}
	if status, ok := filters["status"]; ok {
		query = query.Where("status = ?", status)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询列表
	err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&tasks).Error
	if err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}
