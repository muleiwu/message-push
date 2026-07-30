package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	internalHelper "cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"gorm.io/gorm"
)

// TerminalTransition describes one final task state change and its public event.
type TerminalTransition struct {
	TaskID         string
	Status         string
	Event          string
	ErrorCode      string
	ErrorMessage   string
	ProviderID     string
	OccurredAt     time.Time
	CallbackStatus string
	CallbackTime   *time.Time
}

type TerminalTransitionResult struct {
	Changed  bool
	OutboxID uint
}

// UpstreamEvent is persisted together with its callback log.
type UpstreamEvent struct {
	AppID        string
	Mobile       string
	Content      string
	ProviderCode string
	ReceiveTime  time.Time
	RawData      string
}

// TaskTerminalService owns terminal task transitions and durable webhook events.
type TaskTerminalService struct {
	db  *gorm.DB
	now func() time.Time
}

func NewTaskTerminalService() *TaskTerminalService {
	return NewTaskTerminalServiceWithDB(internalHelper.GetDatabase())
}

func NewTaskTerminalServiceWithDB(db *gorm.DB) *TaskTerminalService {
	return &TaskTerminalService{
		db:  db,
		now: time.Now,
	}
}

func (s *TaskTerminalService) Transition(ctx context.Context, transition TerminalTransition) (*TerminalTransitionResult, error) {
	if err := validateTerminalTransition(transition); err != nil {
		return nil, err
	}
	if transition.OccurredAt.IsZero() {
		transition.OccurredAt = s.now()
	}
	persistedAt := s.now()

	result := &TerminalTransitionResult{}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"status":     transition.Status,
			"updated_at": persistedAt,
		}
		if transition.CallbackStatus != "" {
			updates["callback_status"] = transition.CallbackStatus
		}
		if transition.CallbackTime != nil {
			updates["callback_time"] = *transition.CallbackTime
		}

		updateResult := tx.Model(&model.PushTask{}).
			Where("task_id = ?", transition.TaskID).
			Where("status NOT IN ?", []string{constants.TaskStatusSuccess, constants.TaskStatusFailed}).
			Updates(updates)
		if updateResult.Error != nil {
			return updateResult.Error
		}
		if updateResult.RowsAffected == 0 {
			return nil
		}

		var task model.PushTask
		if err := tx.Where("task_id = ?", transition.TaskID).First(&task).Error; err != nil {
			return err
		}

		outbox, err := s.createOutboxTx(tx, &task, transition, persistedAt)
		if err != nil {
			return err
		}
		result.Changed = true
		if outbox != nil {
			result.OutboxID = outbox.ID
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RecordUpstream stores the provider callback and the upstream webhook event atomically.
func (s *TaskTerminalService) RecordUpstream(ctx context.Context, event UpstreamEvent) error {
	if event.ReceiveTime.IsZero() {
		event.ReceiveTime = s.now()
	}
	persistedAt := s.now()

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		callbackLog := &model.CallbackLog{
			Type:         constants.CallbackTypeUpstream,
			AppID:        event.AppID,
			ProviderCode: event.ProviderCode,
			Mobile:       event.Mobile,
			Content:      event.Content,
			RawData:      event.RawData,
			CreatedAt:    event.ReceiveTime,
		}
		if err := tx.Create(callbackLog).Error; err != nil {
			return err
		}
		if event.AppID == "" {
			return nil
		}

		config, enabled, err := resolveWebhookConfigTx(tx, event.AppID, constants.WebhookEventUpstream)
		if err != nil || !enabled {
			return err
		}

		payload := &dto.WebhookPayload{
			Event:     constants.WebhookEventUpstream,
			AppID:     event.AppID,
			Receiver:  event.Mobile,
			Timestamp: event.ReceiveTime.Unix(),
			Extra: map[string]interface{}{
				"mobile":        event.Mobile,
				"content":       event.Content,
				"provider_code": event.ProviderCode,
				"receive_time":  event.ReceiveTime.Format(time.RFC3339),
			},
		}
		return createWebhookOutboxTx(
			tx,
			fmt.Sprintf("callback:%d:upstream", callbackLog.ID),
			"",
			event.AppID,
			config,
			payload,
			persistedAt,
		)
	})
}

func (s *TaskTerminalService) createOutboxTx(
	tx *gorm.DB,
	task *model.PushTask,
	transition TerminalTransition,
	persistedAt time.Time,
) (*model.WebhookLog, error) {
	config, enabled, err := resolveWebhookConfigTx(tx, task.AppID, transition.Event)
	if err != nil || !enabled {
		return nil, err
	}

	payload := &dto.WebhookPayload{
		Event:     transition.Event,
		TaskID:    task.TaskID,
		AppID:     task.AppID,
		Status:    transition.Status,
		Receiver:  task.Receiver,
		ErrorCode: transition.ErrorCode,
		ErrorMsg:  transition.ErrorMessage,
		Timestamp: transition.OccurredAt.Unix(),
		Extra: map[string]interface{}{
			"provider_id": transition.ProviderID,
			"report_time": transition.OccurredAt.Format(time.RFC3339),
		},
	}

	outbox := &model.WebhookLog{}
	if err := createWebhookOutboxTx(
		tx,
		"task:"+task.TaskID+":terminal",
		task.TaskID,
		task.AppID,
		config,
		payload,
		persistedAt,
		outbox,
	); err != nil {
		return nil, err
	}
	return outbox, nil
}

type webhookConfigSnapshot struct {
	ID             uint
	URL            string
	Secret         string
	MaxRetries     int
	TimeoutSeconds int
}

func resolveWebhookConfigTx(tx *gorm.DB, appID, event string) (*webhookConfigSnapshot, bool, error) {
	var config model.WebhookConfig
	configResult := tx.Where("app_id = ?", appID).Limit(1).Find(&config)
	if configResult.Error != nil {
		return nil, false, configResult.Error
	}
	if configResult.RowsAffected > 0 {
		if !config.IsEnabled() || !config.ShouldNotify(event) {
			return nil, false, nil
		}
		return normalizeWebhookConfig(&config), true, nil
	}

	var app model.Application
	appResult := tx.Where("app_id = ?", appID).Limit(1).Find(&app)
	if appResult.Error != nil {
		return nil, false, appResult.Error
	}
	if appResult.RowsAffected == 0 {
		return nil, false, nil
	}
	if strings.TrimSpace(app.WebhookURL) == "" || !containsWebhookEvent(constants.DefaultWebhookEvents, event) {
		return nil, false, nil
	}
	return &webhookConfigSnapshot{
		URL:            strings.TrimSpace(app.WebhookURL),
		MaxRetries:     constants.DefaultWebhookRetries,
		TimeoutSeconds: constants.DefaultWebhookTimeout,
	}, true, nil
}

func normalizeWebhookConfig(config *model.WebhookConfig) *webhookConfigSnapshot {
	retries := config.RetryCount
	if retries <= 0 {
		retries = constants.DefaultWebhookRetries
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = constants.DefaultWebhookTimeout
	}
	return &webhookConfigSnapshot{
		ID:             config.ID,
		URL:            config.WebhookURL,
		Secret:         config.Secret,
		MaxRetries:     retries,
		TimeoutSeconds: timeout,
	}
}

func createWebhookOutboxTx(
	tx *gorm.DB,
	dedupKey, taskID, appID string,
	config *webhookConfigSnapshot,
	payload *dto.WebhookPayload,
	persistedAt time.Time,
	destinations ...*model.WebhookLog,
) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}
	outbox := &model.WebhookLog{}
	if len(destinations) > 0 && destinations[0] != nil {
		outbox = destinations[0]
	}
	*outbox = model.WebhookLog{
		TaskID:          taskID,
		AppID:           appID,
		WebhookConfigID: config.ID,
		WebhookURL:      config.URL,
		Event:           payload.Event,
		RequestData:     string(body),
		Status:          constants.WebhookDeliveryPending,
		DedupKey:        dedupKey,
		SigningSecret:   config.Secret,
		MaxRetries:      config.MaxRetries,
		TimeoutSeconds:  config.TimeoutSeconds,
		NextAttemptAt:   &persistedAt,
		CreatedAt:       persistedAt,
		UpdatedAt:       persistedAt,
	}
	return tx.Create(outbox).Error
}

func validateTerminalTransition(transition TerminalTransition) error {
	if transition.TaskID == "" {
		return fmt.Errorf("task_id is required")
	}
	if transition.Status != constants.TaskStatusSuccess && transition.Status != constants.TaskStatusFailed {
		return fmt.Errorf("invalid terminal status: %s", transition.Status)
	}
	switch transition.Event {
	case constants.WebhookEventSuccess, constants.WebhookEventDelivered:
		if transition.Status != constants.TaskStatusSuccess {
			return fmt.Errorf("event %s requires success status", transition.Event)
		}
		return nil
	case constants.WebhookEventFailed, constants.WebhookEventRejected:
		if transition.Status != constants.TaskStatusFailed {
			return fmt.Errorf("event %s requires failed status", transition.Event)
		}
		return nil
	default:
		return fmt.Errorf("invalid terminal webhook event: %s", transition.Event)
	}
}

func containsWebhookEvent(events, event string) bool {
	for _, candidate := range strings.Split(events, ",") {
		if strings.TrimSpace(candidate) == event {
			return true
		}
	}
	return false
}
