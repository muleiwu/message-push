package dao

import (
	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/dto"
	apphelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"gorm.io/gorm"
)

// PushLogDAO 推送日志DAO
type PushLogDAO struct {
	db *gorm.DB
}

// NewPushLogDAO 创建DAO
func NewPushLogDAO() *PushLogDAO {
	return &PushLogDAO{
		db: helper.GetDatabase(),
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

// UpdateStatus 根据ID更新日志状态与错误信息
// 用于回执回调回写供应商发送记录的最终投递结果（success/failed）
func (d *PushLogDAO) UpdateStatus(id uint, status, errorMessage string) error {
	updates := map[string]interface{}{"status": status}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	return d.db.Model(&model.PushLog{}).Where("id = ?", id).Updates(updates).Error
}

// GetByProviderMsgID 根据服务商消息ID获取日志
func (d *PushLogDAO) GetByProviderMsgID(providerMsgID string) (*model.PushLog, error) {
	var log model.PushLog
	err := d.db.Where("provider_msg_id = ?", providerMsgID).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// GetByProviderMsgIDAndReceiver 根据服务商消息ID + 接收方手机号获取日志
// 批量发送时整批共用同一个 provider_msg_id（非唯一），需结合回执携带的手机号定位到具体任务。
// 通过 push_tasks.receiver 关联，取最新一条匹配的发送日志。
// 任务 receiver 存的是调用方传入的原始格式（+86 前缀、裸号码、0086 等），而回执回传的是
// 发送时按服务商规则格式化后的号码，两者可能不一致，因此按同一号码的等价格式集合匹配。
func (d *PushLogDAO) GetByProviderMsgIDAndReceiver(providerMsgID, receiver string) (*model.PushLog, error) {
	var log model.PushLog
	err := d.db.
		Joins("JOIN push_tasks ON push_tasks.task_id = push_logs.task_id").
		Where("push_logs.provider_msg_id = ? AND push_tasks.receiver IN ?", providerMsgID, receiverCandidates(receiver)).
		Order("push_logs.id DESC").
		First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// receiverCandidates 生成同一手机号的等价书写格式集合，用于回执手机号与任务 receiver 的匹配。
// 解析失败时仅返回原值。
func receiverCandidates(receiver string) []string {
	candidates := []string{receiver}
	info := apphelper.ParsePhoneNumber(receiver)
	if !info.Valid {
		return candidates
	}
	for _, v := range []string{
		info.E164,                                          // +8618751973856
		info.NationalNumber,                                // 18751973856
		info.CountryCode + info.NationalNumber,             // 8618751973856
		"00" + info.CountryCode + info.NationalNumber,      // 008618751973856
		"+" + info.CountryCode + "-" + info.NationalNumber, // +86-18751973856（网易非大陆发送格式）
	} {
		if v != receiver {
			candidates = append(candidates, v)
		}
	}
	return candidates
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
