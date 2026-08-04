package infrastructure

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	internalHelper "cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/timeutil"
	"github.com/google/uuid"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
)

const (
	webhookDispatchInterval = time.Second
	webhookDispatchBatch    = 100
	webhookDispatchWorkers  = 10
	maxWebhookResponseBytes = 1 << 20
)

type WebhookDispatcher struct {
	logger    gsr.Logger
	logDAO    *dao.WebhookLogDAO
	httpDoer  func(timeout time.Duration) webhookHTTPDoer
	now       func() time.Time
	newToken  func() string
	semaphore chan struct{}
	wg        sync.WaitGroup
}

type webhookHTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewWebhookDispatcher() *WebhookDispatcher {
	return NewWebhookDispatcherWithDB(internalHelper.GetDatabase(), internalHelper.GetLogger())
}

func NewWebhookDispatcherWithDB(db *gorm.DB, logger gsr.Logger) *WebhookDispatcher {
	return &WebhookDispatcher{
		logger: logger,
		logDAO: dao.NewWebhookLogDAOWithDB(db),
		httpDoer: func(timeout time.Duration) webhookHTTPDoer {
			return &http.Client{Timeout: timeout}
		},
		now:       timeutil.Now,
		newToken:  func() string { return uuid.NewString() },
		semaphore: make(chan struct{}, webhookDispatchWorkers),
	}
}

func (d *WebhookDispatcher) Start(ctx context.Context) {
	d.logger.Info("webhook dispatcher started")
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		ticker := time.NewTicker(webhookDispatchInterval)
		defer ticker.Stop()

		d.DispatchOnce(ctx)
		for {
			select {
			case <-ticker.C:
				d.DispatchOnce(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (d *WebhookDispatcher) Stop() {
	d.wg.Wait()
	d.logger.Info("webhook dispatcher stopped")
}

// DispatchOnce scans and claims due rows. It is public to provide a deterministic test seam.
func (d *WebhookDispatcher) DispatchOnce(ctx context.Context) {
	now := timeutil.Normalize(d.now())
	logs, err := d.logDAO.ListDue(ctx, now, webhookDispatchBatch)
	if err != nil {
		if ctx.Err() == nil {
			d.logger.Error(fmt.Sprintf("failed to scan webhook outbox: %v", err))
		}
		return
	}

	for _, webhookLog := range logs {
		timeout := normalizedWebhookTimeout(webhookLog.TimeoutSeconds)
		leaseDuration := timeout + 10*time.Second
		if leaseDuration < 30*time.Second {
			leaseDuration = 30 * time.Second
		}
		token := d.newToken()
		claimed, err := d.logDAO.Claim(ctx, webhookLog.ID, token, now, now.Add(leaseDuration))
		if err != nil {
			d.logger.Error(fmt.Sprintf("failed to claim webhook delivery id=%d: %v", webhookLog.ID, err))
			continue
		}
		if !claimed {
			continue
		}

		select {
		case d.semaphore <- struct{}{}:
		case <-ctx.Done():
			d.releaseClaim(webhookLog, token, ctx.Err())
			return
		}
		d.wg.Add(1)
		go func(log *model.WebhookLog, leaseToken string) {
			defer d.wg.Done()
			defer func() { <-d.semaphore }()
			d.dispatch(ctx, log, leaseToken)
		}(webhookLog, token)
	}
}

func (d *WebhookDispatcher) dispatch(ctx context.Context, webhookLog *model.WebhookLog, token string) {
	responseStatus, responseData, sendErr := d.send(ctx, webhookLog)
	now := timeutil.Normalize(d.now())
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if sendErr == nil {
		if err := d.logDAO.MarkSuccess(
			dbCtx,
			webhookLog.ID,
			token,
			responseStatus,
			responseData,
			now,
		); err != nil {
			d.logger.Error(fmt.Sprintf("failed to mark webhook delivery successful id=%d: %v", webhookLog.ID, err))
			return
		}
		d.logger.Info(fmt.Sprintf("webhook delivered id=%d task_id=%s event=%s", webhookLog.ID, webhookLog.TaskID, webhookLog.Event))
		return
	}

	if webhookLog.RetryCount >= webhookLog.MaxRetries {
		if err := d.logDAO.MarkFailed(
			dbCtx,
			webhookLog.ID,
			token,
			responseStatus,
			responseData,
			sendErr.Error(),
			now,
		); err != nil {
			d.logger.Error(fmt.Sprintf("failed to mark webhook delivery failed id=%d: %v", webhookLog.ID, err))
			return
		}
		d.logger.Error(fmt.Sprintf("webhook delivery exhausted retries id=%d task_id=%s error=%v", webhookLog.ID, webhookLog.TaskID, sendErr))
		return
	}

	retryCount := webhookLog.RetryCount + 1
	nextAttemptAt := now.Add(time.Duration(retryCount) * time.Second)
	if err := d.logDAO.MarkRetry(
		dbCtx,
		webhookLog.ID,
		token,
		retryCount,
		nextAttemptAt,
		responseStatus,
		responseData,
		sendErr.Error(),
		now,
	); err != nil {
		d.logger.Error(fmt.Sprintf("failed to reschedule webhook delivery id=%d: %v", webhookLog.ID, err))
		return
	}
	d.logger.Warn(fmt.Sprintf("webhook delivery scheduled for retry id=%d retry=%d next=%s error=%v",
		webhookLog.ID, retryCount, timeutil.FormatRFC3339(nextAttemptAt), sendErr))
}

func (d *WebhookDispatcher) send(ctx context.Context, webhookLog *model.WebhookLog) (int, string, error) {
	timeout := normalizedWebhookTimeout(webhookLog.TimeoutSeconds)
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodPost,
		webhookLog.WebhookURL,
		bytes.NewBufferString(webhookLog.RequestData),
	)
	if err != nil {
		return 0, "", fmt.Errorf("create webhook request: %w", err)
	}

	attemptTimestamp := timeutil.Normalize(d.now()).Unix()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "MessagePush-Webhook/1.0")
	req.Header.Set("X-Webhook-Event", webhookLog.Event)
	req.Header.Set("X-Webhook-Timestamp", strconv.FormatInt(attemptTimestamp, 10))
	req.Header.Set("X-Webhook-Delivery-ID", strconv.FormatUint(uint64(webhookLog.ID), 10))
	if webhookLog.SigningSecret != "" {
		req.Header.Set(
			"X-Webhook-Signature",
			generateWebhookSignature([]byte(webhookLog.RequestData), webhookLog.SigningSecret, attemptTimestamp),
		)
	}

	resp, err := d.httpDoer(timeout).Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("send webhook request: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxWebhookResponseBytes))
	responseData := string(body)
	if readErr != nil {
		return resp.StatusCode, responseData, fmt.Errorf("read webhook response: %w", readErr)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return resp.StatusCode, responseData, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return resp.StatusCode, responseData, nil
}

func (d *WebhookDispatcher) releaseClaim(webhookLog *model.WebhookLog, token string, cause error) {
	now := timeutil.Normalize(d.now())
	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.logDAO.MarkRetry(
		dbCtx,
		webhookLog.ID,
		token,
		webhookLog.RetryCount,
		now,
		webhookLog.ResponseStatus,
		webhookLog.ResponseData,
		cause.Error(),
		now,
	); err != nil {
		d.logger.Error(fmt.Sprintf("failed to release webhook claim id=%d: %v", webhookLog.ID, err))
	}
}

func normalizedWebhookTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = constants.DefaultWebhookTimeout
	}
	return time.Duration(seconds) * time.Second
}

func generateWebhookSignature(body []byte, secret string, timestamp int64) string {
	message := fmt.Sprintf("%d.%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
