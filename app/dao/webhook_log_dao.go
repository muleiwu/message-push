package dao

import (
	"context"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/timeutil"
	"gorm.io/gorm"
)

// WebhookLogDAO Webhook日志数据访问对象
type WebhookLogDAO struct {
	db *gorm.DB
}

// NewWebhookLogDAO 创建 WebhookLogDAO
func NewWebhookLogDAO() *WebhookLogDAO {
	return NewWebhookLogDAOWithDB(helper.GetDatabase())
}

func NewWebhookLogDAOWithDB(db *gorm.DB) *WebhookLogDAO {
	return &WebhookLogDAO{db: db}
}

// Create 创建Webhook日志
func (dao *WebhookLogDAO) Create(log *model.WebhookLog) error {
	log.NextAttemptAt = timeutil.NormalizePtr(log.NextAttemptAt)
	log.LockedUntil = timeutil.NormalizePtr(log.LockedUntil)
	log.CreatedAt = timeutil.Normalize(log.CreatedAt)
	log.UpdatedAt = timeutil.Normalize(log.UpdatedAt)
	return dao.db.Create(log).Error
}

// GetByTaskID 根据任务ID获取Webhook日志
func (dao *WebhookLogDAO) GetByTaskID(taskID string) ([]*model.WebhookLog, error) {
	var logs []*model.WebhookLog
	err := dao.db.Where("task_id = ?", taskID).
		Order("created_at DESC").
		Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByWebhookConfigID 根据Webhook配置ID获取日志
func (dao *WebhookLogDAO) GetByWebhookConfigID(configID uint) ([]*model.WebhookLog, error) {
	var logs []*model.WebhookLog
	err := dao.db.Where("webhook_config_id = ?", configID).
		Order("created_at DESC").
		Find(&logs).Error
	if err != nil {
		return nil, err
	}
	return logs, nil
}

// List 获取Webhook日志列表（分页）
func (dao *WebhookLogDAO) List(appID string, status string, page, pageSize int) ([]*model.WebhookLog, int64, error) {
	var logs []*model.WebhookLog
	var total int64

	query := dao.db.Model(&model.WebhookLog{})
	if appID != "" {
		query = query.Where("app_id = ?", appID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
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

// ListDue returns pending deliveries and expired processing leases.
func (dao *WebhookLogDAO) ListDue(ctx context.Context, now time.Time, limit int) ([]*model.WebhookLog, error) {
	now = timeutil.Normalize(now)
	var logs []*model.WebhookLog
	err := dao.db.WithContext(ctx).
		Where(
			"(status = ? AND next_attempt_at <= ?) OR (status = ? AND locked_until <= ?)",
			constants.WebhookDeliveryPending,
			now,
			constants.WebhookDeliveryProcessing,
			now,
		).
		Order("next_attempt_at ASC, id ASC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

// Claim obtains a lease using a conditional update so multiple instances can scan safely.
func (dao *WebhookLogDAO) Claim(
	ctx context.Context,
	id uint,
	token string,
	now, lockedUntil time.Time,
) (bool, error) {
	now = timeutil.Normalize(now)
	lockedUntil = timeutil.Normalize(lockedUntil)
	result := dao.db.WithContext(ctx).
		Model(&model.WebhookLog{}).
		Where("id = ?", id).
		Where(
			"(status = ? AND next_attempt_at <= ?) OR (status = ? AND locked_until <= ?)",
			constants.WebhookDeliveryPending,
			now,
			constants.WebhookDeliveryProcessing,
			now,
		).
		Updates(map[string]interface{}{
			"status":       constants.WebhookDeliveryProcessing,
			"lease_token":  token,
			"locked_until": lockedUntil,
			"updated_at":   now,
		})
	return result.RowsAffected == 1, result.Error
}

func (dao *WebhookLogDAO) MarkSuccess(
	ctx context.Context,
	id uint,
	token string,
	responseStatus int,
	responseData string,
	now time.Time,
) error {
	now = timeutil.Normalize(now)
	return dao.db.WithContext(ctx).
		Model(&model.WebhookLog{}).
		Where("id = ? AND status = ? AND lease_token = ?", id, constants.WebhookDeliveryProcessing, token).
		Updates(map[string]interface{}{
			"status":          constants.WebhookDeliverySuccess,
			"response_status": responseStatus,
			"response_data":   responseData,
			"error_message":   "",
			"next_attempt_at": nil,
			"locked_until":    nil,
			"lease_token":     "",
			"updated_at":      now,
		}).Error
}

func (dao *WebhookLogDAO) MarkRetry(
	ctx context.Context,
	id uint,
	token string,
	retryCount int,
	nextAttemptAt time.Time,
	responseStatus int,
	responseData, errorMessage string,
	now time.Time,
) error {
	nextAttemptAt = timeutil.Normalize(nextAttemptAt)
	now = timeutil.Normalize(now)
	return dao.db.WithContext(ctx).
		Model(&model.WebhookLog{}).
		Where("id = ? AND status = ? AND lease_token = ?", id, constants.WebhookDeliveryProcessing, token).
		Updates(map[string]interface{}{
			"status":          constants.WebhookDeliveryPending,
			"retry_count":     retryCount,
			"next_attempt_at": nextAttemptAt,
			"response_status": responseStatus,
			"response_data":   responseData,
			"error_message":   errorMessage,
			"locked_until":    nil,
			"lease_token":     "",
			"updated_at":      now,
		}).Error
}

func (dao *WebhookLogDAO) MarkFailed(
	ctx context.Context,
	id uint,
	token string,
	responseStatus int,
	responseData, errorMessage string,
	now time.Time,
) error {
	now = timeutil.Normalize(now)
	return dao.db.WithContext(ctx).
		Model(&model.WebhookLog{}).
		Where("id = ? AND status = ? AND lease_token = ?", id, constants.WebhookDeliveryProcessing, token).
		Updates(map[string]interface{}{
			"status":          constants.WebhookDeliveryFailed,
			"response_status": responseStatus,
			"response_data":   responseData,
			"error_message":   errorMessage,
			"next_attempt_at": nil,
			"locked_until":    nil,
			"lease_token":     "",
			"updated_at":      now,
		}).Error
}
