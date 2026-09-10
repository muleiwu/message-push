package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
	senderdomain "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *SMSEventService) GetPolling(ctx context.Context, accountID uint) (*SMSPollingResponse, error) {
	_, appID, err := s.account(ctx, accountID, false)
	if err != nil {
		return nil, err
	}
	result := &SMSPollingResponse{Streams: []model.SMSPollingStream{}, Runs: []model.SMSPullRun{}}
	if err := s.db.WithContext(ctx).Where("provider_account_id = ?", accountID).Order("kind").Find(&result.Streams).Error; err != nil {
		return nil, err
	}
	for _, kind := range []string{senderdomain.SMSReports, senderdomain.SMSReplies} {
		found := false
		for _, stream := range result.Streams {
			if stream.Kind == kind {
				found = true
			}
		}
		if !found {
			result.Streams = append(result.Streams, model.SMSPollingStream{ProviderAccountID: accountID, SDKAppID: appID, Kind: kind, IntervalSeconds: 30})
		}
	}
	if err := s.db.WithContext(ctx).Where("provider_account_id = ?", accountID).Order("id DESC").Limit(20).Find(&result.Runs).Error; err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SMSEventService) UpdatePolling(ctx context.Context, accountID uint, req SMSPollingRequest) (*SMSPollingResponse, error) {
	if req.IntervalSeconds == 0 {
		req.IntervalSeconds = 30
	}
	if req.IntervalSeconds < 10 || req.IntervalSeconds > 3600 {
		return nil, fmt.Errorf("%w：轮询间隔须为 10～3600 秒", ErrSMSInvalid)
	}
	_, appID, err := s.account(ctx, accountID, req.ReportsEnabled || req.RepliesEnabled)
	if err != nil {
		return nil, err
	}
	unlock, err := s.acquire(ctx, appID+":settings")
	if err != nil {
		return nil, err
	}
	defer unlock()
	if err := s.ensureStreams(ctx, accountID, appID); err != nil {
		return nil, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, kind := range []string{senderdomain.SMSReports, senderdomain.SMSReplies} {
			enabled, resume := req.ReportsEnabled, req.ResumeReports
			if kind == senderdomain.SMSReplies {
				enabled, resume = req.RepliesEnabled, req.ResumeReplies
			}
			var row model.SMSPollingStream
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("provider_account_id = ? AND kind = ?", accountID, kind).First(&row).Error; err != nil {
				return err
			}
			var ownerKey *string
			if enabled {
				key := appID + ":" + kind
				var occupied int64
				if err := tx.Model(&model.SMSPollingStream{}).Where("owner_key = ? AND provider_account_id <> ?", key, accountID).Count(&occupied).Error; err != nil {
					return err
				}
				if occupied > 0 {
					return fmt.Errorf("%w：同一 SmsSdkAppId 的此类队列已在其他本地账号启用", ErrSMSBusy)
				}
				ownerKey = &key
			}
			if resume {
				var running int64
				if err := tx.Model(&model.SMSPullRun{}).Where("sdk_app_id = ? AND kind = ? AND source <> ? AND state = ?", appID, kind, "query", "started").Count(&running).Error; err != nil {
					return err
				}
				if running != 0 {
					return ErrSMSBusy
				}
				row.Suspended, row.SuspendReason, row.Failures, row.LastError = false, "", 0, ""
			}
			row.SDKAppID, row.Enabled, row.OwnerKey, row.IntervalSeconds, row.UpdatedAt = appID, enabled, ownerKey, req.IntervalSeconds, s.now()
			next := s.now()
			row.NextPollAt = &next
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetPolling(ctx, accountID)
}

// RecoverInterruptedPulls is safe on every scan. Stale queue reads are paused,
// not replayed; historical queries can simply be submitted again by an admin.
func (s *SMSEventService) RecoverInterruptedPulls(ctx context.Context) error {
	var runs []model.SMSPullRun
	if err := s.db.WithContext(ctx).Where("state = ? AND started_at < ?", "started", s.now().Add(-60*time.Second)).Limit(100).Find(&runs).Error; err != nil {
		return err
	}
	for _, run := range runs {
		if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			state := "uncertain"
			if run.Source == "query" {
				state = "failed"
			}
			message := "上次执行中断，结果不确定；请核对执行记录，必要时按号码补录后恢复"
			updated := tx.Model(&model.SMSPullRun{}).Where("id = ? AND state = ?", run.ID, "started").Updates(map[string]any{"state": state, "error_code": "INTERRUPTED", "error_message": message, "finished_at": s.now()})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected == 0 || run.Source == "query" {
				return nil
			}
			return tx.Model(&model.SMSPollingStream{}).Where("sdk_app_id = ? AND kind = ?", run.SDKAppID, run.Kind).Updates(map[string]any{"suspended": true, "suspend_reason": message, "last_error": message, "last_run_id": run.ID, "updated_at": s.now()}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *SMSEventService) PollDue(ctx context.Context) error {
	if err := s.RecoverInterruptedPulls(ctx); err != nil {
		return err
	}
	var streams []model.SMSPollingStream
	if err := s.db.WithContext(ctx).Where("enabled = ? AND suspended = ? AND (next_poll_at IS NULL OR next_poll_at <= ?)", true, false, s.now()).Order("next_poll_at").Limit(20).Find(&streams).Error; err != nil {
		return err
	}
	for _, stream := range streams {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, appID, err := s.account(ctx, stream.ProviderAccountID, true)
		if err != nil && !errors.Is(err, ErrSMSInvalid) && !errors.Is(err, ErrSMSNotFound) {
			return err
		}
		if err != nil || appID != stream.SDKAppID {
			reason := "账号已禁用、删除或配置变化，请检查后恢复轮询"
			if err := s.db.WithContext(ctx).Model(&stream).Updates(map[string]any{"suspended": true, "owner_key": nil, "suspend_reason": reason, "last_error": reason, "updated_at": s.now()}).Error; err != nil {
				return err
			}
			continue
		}
		runCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		for batch := 0; batch < 5; batch++ {
			// Leave a full RPC timeout plus persistence time before starting
			// another irreversible consumption near the round deadline.
			if deadline, ok := runCtx.Deadline(); ok && time.Until(deadline) < 15*time.Second {
				break
			}
			result, err := s.Collect(runCtx, stream.ProviderAccountID, stream.Kind, "poll", SMSQueryRequest{Limit: 100})
			if err != nil {
				if !errors.Is(err, ErrSMSBusy) && !errors.Is(err, ErrSMSPaused) {
					s.logger.Warn("腾讯云短信轮询失败：" + err.Error())
				}
				break
			}
			if !result.LimitReached {
				break
			}
		}
		cancel()
	}
	return nil
}

type SMSListRequest struct {
	Kind        string `form:"kind"`
	State       string `form:"state"`
	Source      string `form:"source"`
	Mobile      string `form:"mobile"`
	Attribution string `form:"attribution"`
	Page        int    `form:"page"`
	PageSize    int    `form:"page_size"`
}

type SMSEventList struct {
	Items    []model.ProviderSMSEvent `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
}

func (s *SMSEventService) ListEvents(ctx context.Context, accountID uint, req SMSListRequest) (*SMSEventList, error) {
	if _, _, err := s.account(ctx, accountID, false); err != nil {
		return nil, err
	}
	if req.Kind != "" && !smsKindValid(req.Kind) {
		return nil, ErrSMSInvalid
	}
	if req.Page == 0 {
		req.Page = 1
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	if req.Page < 1 || req.PageSize < 1 || req.PageSize > 100 {
		return nil, ErrSMSInvalid
	}
	q := s.db.WithContext(ctx).Model(&model.ProviderSMSEvent{}).Where("provider_account_id = ?", accountID)
	for column, value := range map[string]string{"kind": req.Kind, "state": req.State, "source": req.Source, "mobile": req.Mobile, "attribution": req.Attribution} {
		if value != "" {
			q = q.Where(column+" = ?", value)
		}
	}
	result := &SMSEventList{Items: []model.ProviderSMSEvent{}, Page: req.Page, PageSize: req.PageSize}
	if err := q.Count(&result.Total).Error; err != nil {
		return nil, err
	}
	if err := q.Order("id DESC").Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Find(&result.Items).Error; err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SMSEventService) RetryEvent(ctx context.Context, accountID uint, eventID uint64) error {
	var event model.ProviderSMSEvent
	if err := s.db.WithContext(ctx).Where("id = ? AND provider_account_id = ?", eventID, accountID).First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSMSNotFound
		}
		return err
	}
	if event.State != "failed" {
		return fmt.Errorf("%w：仅处理失败的记录可以重试", ErrSMSInvalid)
	}
	// No reporter/SDK call: retrying refers solely to already saved facts.
	return s.db.WithContext(ctx).Model(&event).Where("state = ?", "failed").Updates(map[string]any{"state": "pending", "next_attempt_at": s.now(), "last_error": "", "updated_at": s.now()}).Error
}
