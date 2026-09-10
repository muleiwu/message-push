package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	apphelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/delivery"
	deliverydomain "cnb.cool/mliev/push/message-push/modules/delivery/domain"
	"cnb.cool/mliev/push/message-push/modules/ruleengine"
	senderdomain "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type smsDeliveryEffect struct {
	TaskID     string    `json:"task_id"`
	RetryCount int       `json:"retry_count"`
	At         time.Time `json:"at"`
}

type smsEffectRecorder struct {
	delivery.Producer
	effect *smsDeliveryEffect
	now    func() time.Time
}

func (p *smsEffectRecorder) Push(_ context.Context, task *model.PushTask) error {
	p.effect = &smsDeliveryEffect{TaskID: task.TaskID, RetryCount: task.RetryCount, At: p.now()}
	return nil
}
func (p *smsEffectRecorder) PushDelayed(_ context.Context, task *model.PushTask, at time.Time) error {
	p.effect = &smsDeliveryEffect{TaskID: task.TaskID, RetryCount: task.RetryCount, At: at}
	return nil
}

func (s *SMSEventService) ProcessPending(ctx context.Context) error {
	var events []model.ProviderSMSEvent
	if err := s.db.WithContext(ctx).Select("id").Where("state IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", []string{"pending", "failed", "effect_pending"}, s.now()).Order("id").Limit(100).Find(&events).Error; err != nil {
		return err
	}
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := s.ProcessEvent(ctx, event.ID); err != nil && !errors.Is(err, ErrSMSBusy) {
			s.logger.Warn(fmt.Sprintf("短信事件 %d 处理失败：%v", event.ID, err))
		}
	}
	return nil
}

func (s *SMSEventService) ProcessEvent(ctx context.Context, id uint64) error {
	unlock, err := s.acquire(ctx, fmt.Sprintf("event:%d", id))
	if err != nil {
		return err
	}
	defer unlock()
	var event model.ProviderSMSEvent
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&event, id).Error; err != nil {
			return err
		}
		if event.State != "pending" && event.State != "failed" && event.State != "effect_pending" {
			return nil
		}
		if event.Effect != "" {
			return nil
		} // Decision already committed; retry only its delivery effect.
		event.Attempts++
		event.LastError, event.State = "", "processed"
		phone := apphelper.ParsePhoneNumber(event.Mobile)
		if !phone.Valid || phone.CountryCode != "86" {
			event.State, event.LastError = "invalid", "记录缺少有效的国内号码，原始响应已保留"
		} else if event.Kind == senderdomain.SMSReplies {
			if err := s.processSMSReply(ctx, tx, &event); err != nil {
				return err
			}
		} else if event.Kind == senderdomain.SMSReports {
			if err := s.processSMSReport(ctx, tx, &event); err != nil {
				return err
			}
		} else {
			event.State, event.LastError = "invalid", "未知短信事件类型"
		}
		now := s.now()
		event.UpdatedAt, event.ProcessedAt, event.NextAttemptAt = now, &now, nil
		if event.Effect != "" {
			event.State, event.ProcessedAt, event.NextAttemptAt = "effect_pending", nil, &now
		}
		return tx.Save(&event).Error
	})
	if err == nil && event.Effect != "" && (event.State == "effect_pending" || event.State == "failed" || event.State == "pending") {
		err = s.dispatchSMSEffect(ctx, &event)
	}
	if err != nil && event.ID != 0 {
		next := s.now().Add(time.Duration(1<<min(event.Attempts+1, 8)) * time.Second)
		saveCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if saveErr := s.db.WithContext(saveCtx).Model(&model.ProviderSMSEvent{}).Where("id = ? AND state IN ?", id, []string{"pending", "failed", "effect_pending"}).Updates(map[string]any{"state": "failed", "attempts": gorm.Expr("attempts + 1"), "last_error": err.Error(), "next_attempt_at": next, "updated_at": s.now()}).Error; saveErr != nil {
			s.logger.Error("保存短信处理失败状态失败：" + saveErr.Error())
		}
	}
	return err
}

func (s *SMSEventService) processSMSReply(ctx context.Context, tx *gorm.DB, event *model.ProviderSMSEvent) error {
	if event.OccurredAt == nil || event.Content == "" {
		event.State, event.LastError = "invalid", "回复缺少时间或内容，原始响应已保留"
		return nil
	}
	apps := map[string]bool{}
	after := uint(0)
	for {
		logs, err := dao.NewPushLogDAOWithDB(tx).SMSReplyCandidates(event.ProviderAccountID, event.Mobile, *event.OccurredAt, after)
		if err != nil {
			return err
		}
		for _, log := range logs {
			after = log.ID
			signature := ""
			if log.SendSnapshot != nil {
				var snapshot model.SendSnapshot
				if json.Unmarshal([]byte(*log.SendSnapshot), &snapshot) == nil {
					signature = snapshot.SignatureValue
				}
			}
			if signature == "" {
				var request struct{ SignName string }
				if json.Unmarshal([]byte(log.RequestData), &request) == nil {
					signature = request.SignName
				}
			}
			if event.SignName != "" && signature != "" && normalizeSMSSignature(signature) == normalizeSMSSignature(event.SignName) && log.AppID != "" {
				apps[log.AppID] = true
			}
		}
		if len(apps) > 1 || len(logs) < 200 {
			break
		}
	}
	event.Attribution = "unmatched"
	if len(apps) > 1 {
		event.Attribution = "ambiguous"
	}
	if len(apps) == 1 {
		for app := range apps {
			event.AppID = app
		}
		event.Attribution = "matched"
	}
	return NewTaskTerminalServiceWithDB(tx).RecordUpstream(ctx, UpstreamEvent{AppID: event.AppID, Mobile: event.Mobile, Content: event.Content, ProviderCode: constants.ProviderTencentSMS, ProviderAccountID: event.ProviderAccountID, EventKey: event.EventKey, Source: event.Source, Attribution: event.Attribution, ReceiveTime: *event.OccurredAt, RawData: event.RawData})
}

func normalizeSMSSignature(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "【") && strings.HasSuffix(value, "】") {
		return strings.TrimSuffix(strings.TrimPrefix(value, "【"), "】")
	}
	return value
}

func (s *SMSEventService) processSMSReport(ctx context.Context, tx *gorm.DB, event *model.ProviderSMSEvent) error {
	if event.ProviderMsgID == "" || (event.Status != constants.CallbackStatusDelivered && event.Status != constants.CallbackStatusFailed) {
		event.State, event.LastError = "invalid", "回执缺少流水号或投递结果未知，未改变任务状态"
		return nil
	}
	log, err := dao.NewPushLogDAOWithDB(tx).GetByAccountMsgIDAndReceiver(event.ProviderAccountID, event.ProviderMsgID, event.Mobile)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	callbackLog := &model.CallbackLog{Type: constants.CallbackTypeReport, ProviderAccountID: event.ProviderAccountID, Source: event.Source, SMSEventKey: &event.EventKey, ProviderCode: constants.ProviderTencentSMS, ProviderID: event.ProviderMsgID, Mobile: event.Mobile, CallbackStatus: event.Status, ErrorCode: event.ErrorCode, ErrorMessage: event.ErrorMessage, RawData: event.RawData, CreatedAt: s.now()}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		event.Attribution, callbackLog.Attribution = "unmatched", "unmatched"
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "sms_event_key"}}, DoNothing: true}).Create(callbackLog).Error
	}
	var task model.PushTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("task_id = ?", log.TaskID).First(&task).Error; err != nil {
		return err
	}
	event.AppID, event.Attribution = task.AppID, "matched"
	callbackLog.AppID, callbackLog.TaskID, callbackLog.Attribution = task.AppID, task.TaskID, "matched"
	if event.OccurredAt != nil {
		callbackLog.CreatedAt = *event.OccurredAt
	}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "sms_event_key"}}, DoNothing: true}).Create(callbackLog).Error; err != nil {
		return err
	}
	status := constants.TaskStatusFailed
	if event.Status == constants.CallbackStatusDelivered {
		status = constants.TaskStatusSuccess
	}
	if err := dao.NewPushLogDAOWithDB(tx).UpdateStatus(log.ID, status, event.ErrorMessage); err != nil {
		return err
	}
	// A late receipt for an older attempt cannot restart or finish a newer attempt.
	var latest model.PushLog
	if err := tx.Where("task_id = ? AND provider_msg_id <> ?", task.TaskID, "").Order("id DESC").First(&latest).Error; err != nil {
		return err
	}
	if latest.ID != log.ID || task.Status == constants.TaskStatusSuccess || task.Status == constants.TaskStatusFailed || task.Status == constants.TaskStatusPending {
		return nil
	}
	at := s.now()
	if event.OccurredAt != nil {
		at = *event.OccurredAt
	}
	if event.Status == constants.CallbackStatusDelivered {
		_, err := NewTaskTerminalServiceWithDB(tx).Transition(ctx, TerminalTransition{TaskID: task.TaskID, Status: status, Event: constants.WebhookEventDelivered, ProviderID: event.ProviderMsgID, OccurredAt: at, CallbackStatus: event.Status, CallbackTime: &at})
		return err
	}
	task.CallbackStatus, task.CallbackTime = event.Status, &at
	decision := &ruleengine.EvaluateResult{Action: model.RuleActionFail}
	if s.engine != nil {
		decision = s.engine.Evaluate(ctx, &ruleengine.EvaluateRequest{Scene: model.RuleSceneCallbackFailure, ProviderCode: constants.ProviderTencentSMS, MessageType: constants.MessageTypeSMS, ErrorCode: event.ErrorCode, ErrorMessage: event.ErrorMessage, Task: &task})
	}
	recorder := &smsEffectRecorder{now: s.now}
	executor := &ActionExecutor{logger: s.logger, taskDAO: dao.NewPushTaskDAOWithDB(tx), logDAO: dao.NewPushLogDAOWithDB(tx), producer: recorder, terminalService: NewTaskTerminalServiceWithDB(tx), defaultWebhookURL: s.defaultAlertURL}
	result := executor.Execute(ctx, decision, &ExecuteContext{Task: &task, ProviderAccountID: event.ProviderAccountID, ProviderCode: constants.ProviderTencentSMS, ProviderID: event.ProviderMsgID, ErrorCode: event.ErrorCode, ErrorMessage: event.ErrorMessage, RequestData: event.RawData, TerminalEvent: constants.WebhookEventFailed, OccurredAt: at, CallbackStatus: event.Status, CallbackTime: &at,
		DeferAlert: func(url string, body []byte) error {
			now := s.now()
			key := "sms:" + event.EventKey + ":alert"
			return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "dedup_key"}}, DoNothing: true}).Create(&model.WebhookLog{TaskID: task.TaskID, AppID: task.AppID, WebhookURL: url, Event: "alert", RequestData: string(body), Status: "pending", DedupKey: key, MaxRetries: 3, TimeoutSeconds: 10, NextAttemptAt: &now, CreatedAt: now, UpdatedAt: now}).Error
		},
	})
	if result.Err != nil {
		return result.Err
	}
	if recorder.effect != nil {
		body, err := json.Marshal(recorder.effect)
		if err != nil {
			return err
		}
		event.Effect = string(body)
	}
	return nil
}

func (s *SMSEventService) dispatchSMSEffect(ctx context.Context, event *model.ProviderSMSEvent) error {
	var effect smsDeliveryEffect
	if err := json.Unmarshal([]byte(event.Effect), &effect); err != nil {
		return err
	}
	var task model.PushTask
	if err := s.db.WithContext(ctx).Where("task_id = ?", effect.TaskID).First(&task).Error; err != nil {
		return err
	}
	if task.Status == constants.TaskStatusPending && task.RetryCount == effect.RetryCount {
		producer, ok := s.producer.(deliverydomain.IdempotentProducer)
		if !ok {
			return fmt.Errorf("投递队列不支持事件幂等处理")
		}
		if err := producer.PushDelayedOnce(ctx, &task, effect.At, event.EventKey); err != nil {
			return err
		}
	}
	return s.db.WithContext(ctx).Model(event).Updates(map[string]any{"state": "processed", "processed_at": s.now(), "next_attempt_at": nil, "last_error": "", "updated_at": s.now()}).Error
}
