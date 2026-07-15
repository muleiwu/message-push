package dao

import (
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"gorm.io/gorm"
)

// PushTaskDAO 推送任务数据访问对象
type PushTaskDAO struct {
	db *gorm.DB
}

// NewPushTaskDAO 创建PushTaskDAO
func NewPushTaskDAO() *PushTaskDAO {
	return &PushTaskDAO{
		db: helper.GetDatabase(),
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

// GetLatestReceiverApp 获取最近一次发往该手机号的任务所属应用ID
// 用于上行短信（无 provider_msg_id）尽力关联应用：用户回复通常针对最后一条短信。
// 找不到时返回空字符串、不报错。
// 上行回传的号码格式可能与任务 receiver 的原始格式不一致，按等价格式集合匹配。
func (d *PushTaskDAO) GetLatestReceiverApp(receiver string) (string, error) {
	var task model.PushTask
	err := d.db.Select("app_id").
		Where("receiver IN ?", receiverCandidates(receiver)).
		Order("created_at DESC").
		First(&task).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return task.AppID, nil
}

// IncrementRetryCount 增加重试次数
func (d *PushTaskDAO) IncrementRetryCount(taskID string) error {
	return d.db.Model(&model.PushTask{}).
		Where("task_id = ?", taskID).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

// MarkTimeoutSentTasksCallback 将超时未回调的 sent 状态短信任务的回调状态置为 timeout。
// 条件化更新（CAS）：UPDATE 的 WHERE 复查状态与时间，回调若已并发写入结果则不覆盖；
// 多实例重复执行时 RowsAffected 为 0，天然幂等。
// 分两步（先限量查 ID 再条件更新）是因为 UPDATE ... LIMIT 仅 MySQL 方言支持。
func (d *PushTaskDAO) MarkTimeoutSentTasksCallback(timeout time.Duration, limit int) (int64, error) {
	cutoff := time.Now().Add(-timeout)
	pendingCallback := []string{"", "pending"}

	var ids []uint
	err := d.db.Model(&model.PushTask{}).
		Where("status = ? AND message_type = ? AND updated_at < ?", "sent", "sms", cutoff).
		Where("(callback_status IS NULL OR callback_status IN ?)", pendingCallback).
		Limit(limit).
		Pluck("id", &ids).Error
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	res := d.db.Model(&model.PushTask{}).
		Where("id IN ?", ids).
		Where("status = ? AND updated_at < ?", "sent", cutoff).
		Where("(callback_status IS NULL OR callback_status IN ?)", pendingCallback).
		Updates(map[string]interface{}{
			"callback_status": "timeout",
			"callback_time":   time.Now(),
		})
	return res.RowsAffected, res.Error
}

// MarkTimeoutProcessingTasksFailed 将超时的 processing 状态任务（所有消息类型）标记为失败。
// 条件化更新（CAS）：worker 并发改为 success/sent 的行不会被覆盖；重复执行 RowsAffected 为 0。
func (d *PushTaskDAO) MarkTimeoutProcessingTasksFailed(timeout time.Duration, limit int) (int64, error) {
	cutoff := time.Now().Add(-timeout)

	var ids []uint
	err := d.db.Model(&model.PushTask{}).
		Where("status = ? AND updated_at < ?", "processing", cutoff).
		Limit(limit).
		Pluck("id", &ids).Error
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	res := d.db.Model(&model.PushTask{}).
		Where("id IN ?", ids).
		Where("status = ? AND updated_at < ?", "processing", cutoff).
		Update("status", "failed")
	return res.RowsAffected, res.Error
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
