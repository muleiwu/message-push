package dao

import (
	"time"

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

// GetTimeoutSentTasks 获取超时的 sent 状态短信任务
func (d *PushTaskDAO) GetTimeoutSentTasks(timeout time.Duration, limit int) ([]*model.PushTask, error) {
	var tasks []*model.PushTask
	cutoff := time.Now().Add(-timeout)
	err := d.db.Where("status = ? AND message_type = ? AND updated_at < ?",
		"sent", "sms", cutoff).
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
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
	if messageType, ok := filters["message_type"]; ok {
		query = query.Where("message_type = ?", messageType)
	}
	if taskID, ok := filters["task_id"]; ok {
		query = query.Where("task_id LIKE ?", "%"+taskID.(string)+"%")
	}
	if batchID, ok := filters["batch_id"]; ok {
		query = query.Where("batch_id = ?", batchID)
	}
	if startDate, ok := filters["start_date"]; ok {
		query = query.Where("DATE(created_at) >= ?", startDate)
	}
	if endDate, ok := filters["end_date"]; ok {
		query = query.Where("DATE(created_at) <= ?", endDate)
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
