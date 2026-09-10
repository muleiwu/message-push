package dao

import (
	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/timeutil"
	"gorm.io/gorm"
)

// CallbackLogDAO 回调日志数据访问对象
type CallbackLogDAO struct {
	db *gorm.DB
}

// NewCallbackLogDAO 创建 CallbackLogDAO
func NewCallbackLogDAO() *CallbackLogDAO {
	return NewCallbackLogDAOWithDB(helper.GetDatabase())
}

func NewCallbackLogDAOWithDB(db *gorm.DB) *CallbackLogDAO {
	return &CallbackLogDAO{db: db}
}

// Create 创建回调日志
func (dao *CallbackLogDAO) Create(log *model.CallbackLog) error {
	log.CreatedAt = timeutil.Normalize(log.CreatedAt)
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

// List 获取回调日志列表（支持筛选 + 分页）
func (dao *CallbackLogDAO) List(req *dto.CallbackListRequest) ([]*model.CallbackLog, int64, error) {
	var logs []*model.CallbackLog
	var total int64

	query := dao.db.Model(&model.CallbackLog{})
	if req.ProviderAccountID > 0 {
		query = query.Where("provider_account_id = ?", req.ProviderAccountID)
	}
	if req.Source == "callback" {
		query = query.Where("source = ? OR source = ? OR source IS NULL", "callback", "")
	} else if req.Source != "" {
		query = query.Where("source = ?", req.Source)
	}
	if req.Attribution != "" {
		query = query.Where("attribution = ?", req.Attribution)
	}
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	if req.AppID != "" {
		query = query.Where("app_id = ?", req.AppID)
	}
	if req.Mobile != "" {
		query = query.Where("mobile = ?", req.Mobile)
	}
	query, err := applyBusinessDateFilter(query, "created_at", req.StartDate, req.EndDate)
	if err != nil {
		return nil, 0, err
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (req.Page - 1) * req.PageSize
	err = query.Offset(offset).Limit(req.PageSize).
		Order("created_at DESC").
		Find(&logs).Error

	if err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
