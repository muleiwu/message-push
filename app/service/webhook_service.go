package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/sender"
	internalHelper "cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
)

// WebhookService Webhook 服务
type WebhookService struct {
	logger           gsr.Logger
	webhookConfigDao *dao.WebhookConfigDAO
	httpClient       *http.Client
}

// NewWebhookService 创建 Webhook 服务
func NewWebhookService() *WebhookService {
	h := internalHelper.GetHelper()
	return &WebhookService{
		logger:           h.GetLogger(),
		webhookConfigDao: dao.NewWebhookConfigDAO(),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// WebhookPayload Webhook 推送的数据结构
type WebhookPayload struct {
	Event     string                 `json:"event"`      // 事件类型：success, failed, delivered, rejected
	TaskID    string                 `json:"task_id"`    // 任务ID
	AppID     string                 `json:"app_id"`     // 应用ID
	Status    string                 `json:"status"`     // 任务状态
	Receiver  string                 `json:"receiver"`   // 接收者
	ErrorCode string                 `json:"error_code"` // 错误码（失败时）
	ErrorMsg  string                 `json:"error_msg"`  // 错误信息（失败时）
	Timestamp int64                  `json:"timestamp"`  // 时间戳
	Extra     map[string]interface{} `json:"extra"`      // 额外信息
}

// NotifyStatusChange 通知任务状态变更
func (s *WebhookService) NotifyStatusChange(ctx context.Context, task *model.PushTask, result *sender.CallbackResult) error {
	// 1. 获取应用的 Webhook 配置
	config, err := s.webhookConfigDao.GetEnabledByAppID(task.AppID)
	if err != nil {
		// 没有配置 Webhook，不需要通知
		s.logger.Debug(fmt.Sprintf("no webhook config for app_id=%s", task.AppID))
		return nil
	}

	// 2. 检查是否需要通知此事件
	event := result.Status // delivered, failed, rejected
	if !config.ShouldNotify(event) {
		s.logger.Debug(fmt.Sprintf("event %s not subscribed for app_id=%s", event, task.AppID))
		return nil
	}

	// 3. 构造 Webhook 数据
	payload := &WebhookPayload{
		Event:     event,
		TaskID:    task.TaskID,
		AppID:     task.AppID,
		Status:    task.Status,
		Receiver:  task.Receiver,
		ErrorCode: result.ErrorCode,
		ErrorMsg:  result.ErrorMessage,
		Timestamp: time.Now().Unix(),
		Extra: map[string]interface{}{
			"provider_id": result.ProviderID,
			"report_time": result.ReportTime.Format(time.RFC3339),
		},
	}

	// 4. 发送 Webhook 请求
	return s.sendWebhook(ctx, config, payload)
}

// sendWebhook 发送 Webhook 请求
func (s *WebhookService) sendWebhook(ctx context.Context, config *model.WebhookConfig, payload *WebhookPayload) error {
	// 序列化数据
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "MessagePush-Webhook/1.0")
	req.Header.Set("X-Webhook-Event", payload.Event)
	req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", payload.Timestamp))

	// 如果配置了签名密钥，添加签名
	if config.Secret != "" {
		signature := s.generateSignature(body, config.Secret, payload.Timestamp)
		req.Header.Set("X-Webhook-Signature", signature)
	}

	// 设置超时
	timeout := time.Duration(config.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	s.httpClient.Timeout = timeout

	// 发送请求（带重试）
	var lastErr error
	maxRetries := config.RetryCount
	if maxRetries <= 0 {
		maxRetries = 3
	}

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// 重试延迟
			time.Sleep(time.Duration(i) * time.Second)
			s.logger.Info(fmt.Sprintf("retrying webhook request for task_id=%s, attempt=%d", payload.TaskID, i))
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = err
			s.logger.Error(fmt.Sprintf("webhook request failed: %v", err))
			continue
		}

		// 读取响应
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// 检查响应状态
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			s.logger.Info(fmt.Sprintf("webhook sent successfully task_id=%s url=%s", payload.TaskID, config.WebhookURL))
			return nil
		}

		lastErr = fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(respBody))
		s.logger.Error(fmt.Sprintf("webhook request failed: %v", lastErr))
	}

	return fmt.Errorf("webhook failed after %d retries: %w", maxRetries, lastErr)
}

// generateSignature 生成签名
// 签名算法：HMAC-SHA256(timestamp + "." + body, secret)
func (s *WebhookService) generateSignature(body []byte, secret string, timestamp int64) string {
	message := fmt.Sprintf("%d.%s", timestamp, string(body))
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// NotifyBatchStatus 批量通知任务状态变更
func (s *WebhookService) NotifyBatchStatus(ctx context.Context, tasks []*model.PushTask, results []*sender.CallbackResult) error {
	if len(tasks) != len(results) {
		return fmt.Errorf("tasks and results length mismatch")
	}

	for i, task := range tasks {
		if err := s.NotifyStatusChange(ctx, task, results[i]); err != nil {
			s.logger.Error(fmt.Sprintf("failed to notify webhook for task_id=%s: %v", task.TaskID, err))
			// 继续处理其他任务
		}
	}

	return nil
}
