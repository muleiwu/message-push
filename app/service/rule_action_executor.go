package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/queue"
	internalHelper "cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
)

// ActionExecutor 规则动作执行器
type ActionExecutor struct {
	logger     gsr.Logger
	taskDAO    *dao.PushTaskDAO
	logDAO     *dao.PushLogDAO
	producer   *queue.Producer
	httpClient *http.Client
}

// NewActionExecutor 创建动作执行器
func NewActionExecutor() *ActionExecutor {
	h := internalHelper.GetHelper()
	return &ActionExecutor{
		logger:   h.GetLogger(),
		taskDAO:  dao.NewPushTaskDAO(),
		logDAO:   dao.NewPushLogDAO(),
		producer: queue.NewProducer(h.GetRedis()),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ExecuteContext 执行上下文
type ExecuteContext struct {
	Task              *model.PushTask
	ProviderAccountID uint
	ProviderCode      string
	ErrorCode         string
	ErrorMessage      string
	RequestData       string
	ResponseData      string
}

// ExecuteResult 执行结果
type ExecuteResult struct {
	Action       string        // 执行的动作
	ShouldRetry  bool          // 是否需要重试
	RetryDelay   time.Duration // 重试延迟
	TaskUpdated  bool          // 任务是否已更新
	AlertSent    bool          // 是否已发送告警
	ErrorMessage string        // 错误信息
}

// Execute 执行规则动作
func (e *ActionExecutor) Execute(ctx context.Context, result *EvaluateResult, execCtx *ExecuteContext) *ExecuteResult {
	if result == nil || execCtx == nil || execCtx.Task == nil {
		return &ExecuteResult{
			Action:       model.RuleActionFail,
			ErrorMessage: "invalid execute context",
		}
	}

	switch result.Action {
	case model.RuleActionRetry:
		return e.executeRetry(ctx, result, execCtx)
	case model.RuleActionSwitchProvider:
		return e.executeSwitchProvider(ctx, result, execCtx)
	case model.RuleActionFail:
		return e.executeFail(ctx, result, execCtx)
	case model.RuleActionAlert:
		return e.executeAlert(ctx, result, execCtx)
	default:
		e.logger.Warn(fmt.Sprintf("unknown action=%s, defaulting to fail", result.Action))
		return e.executeFail(ctx, result, execCtx)
	}
}

// executeRetry 执行重试动作
func (e *ActionExecutor) executeRetry(ctx context.Context, result *EvaluateResult, execCtx *ExecuteContext) *ExecuteResult {
	task := execCtx.Task

	// 获取重试配置
	var config *model.RetryActionConfig
	var err error
	if result.MatchedRule != nil {
		config, err = result.MatchedRule.GetRetryConfig()
		if err != nil {
			e.logger.Error(fmt.Sprintf("failed to get retry config: %v", err))
			config = &model.RetryActionConfig{
				MaxRetry:     3,
				DelaySeconds: 2,
				BackoffRate:  2,
				MaxDelay:     60,
			}
		}
	} else {
		// 使用默认配置
		config = &model.RetryActionConfig{
			MaxRetry:     task.MaxRetry,
			DelaySeconds: 2,
			BackoffRate:  2,
			MaxDelay:     60,
		}
	}

	// 检查是否超过最大重试次数
	if task.RetryCount >= config.MaxRetry {
		e.logger.Info(fmt.Sprintf("max retry exceeded, marking as failed task_id=%s retry_count=%d max_retry=%d",
			task.TaskID, task.RetryCount, config.MaxRetry))
		return e.executeFail(ctx, result, execCtx)
	}

	// 计算重试延迟（指数退避）
	delay := e.calculateBackoff(task.RetryCount, config)

	// 更新任务重试次数
	task.RetryCount++
	if err := e.taskDAO.Update(task); err != nil {
		e.logger.Error(fmt.Sprintf("failed to update task retry count: %v", err))
	}

	// 记录重试日志
	e.logDAO.Create(&model.PushLog{
		TaskID:            task.TaskID,
		AppID:             task.AppID,
		ProviderAccountID: execCtx.ProviderAccountID,
		Status:            "retry",
		RequestData:       execCtx.RequestData,
		ResponseData:      execCtx.ResponseData,
		ErrorMessage:      execCtx.ErrorMessage,
	})

	// 延迟后重新推送到队列
	go func() {
		time.Sleep(delay)
		if err := e.producer.Push(context.Background(), task); err != nil {
			e.logger.Error(fmt.Sprintf("failed to push task to queue for retry task_id=%s: %v", task.TaskID, err))
		}
	}()

	e.logger.Info(fmt.Sprintf("task scheduled for retry task_id=%s retry_count=%d delay=%v",
		task.TaskID, task.RetryCount, delay))

	return &ExecuteResult{
		Action:      model.RuleActionRetry,
		ShouldRetry: true,
		RetryDelay:  delay,
		TaskUpdated: true,
	}
}

// executeSwitchProvider 执行切换供应商动作
func (e *ActionExecutor) executeSwitchProvider(ctx context.Context, result *EvaluateResult, execCtx *ExecuteContext) *ExecuteResult {
	task := execCtx.Task

	// 获取切换配置
	var config *model.SwitchProviderActionConfig
	var err error
	if result.MatchedRule != nil {
		config, err = result.MatchedRule.GetSwitchProviderConfig()
		if err != nil {
			e.logger.Error(fmt.Sprintf("failed to get switch provider config: %v", err))
			config = &model.SwitchProviderActionConfig{
				ExcludeCurrent: true,
				MaxRetry:       1,
			}
		}
	} else {
		config = &model.SwitchProviderActionConfig{
			ExcludeCurrent: true,
			MaxRetry:       1,
		}
	}

	// 检查是否超过切换后的最大重试次数
	if task.RetryCount >= config.MaxRetry {
		e.logger.Info(fmt.Sprintf("switch provider max retry exceeded, marking as failed task_id=%s retry_count=%d",
			task.TaskID, task.RetryCount))
		return e.executeFail(ctx, result, execCtx)
	}

	// 更新任务重试次数
	task.RetryCount++
	if err := e.taskDAO.Update(task); err != nil {
		e.logger.Error(fmt.Sprintf("failed to update task for switch provider: %v", err))
	}

	// 记录切换供应商日志
	e.logDAO.Create(&model.PushLog{
		TaskID:            task.TaskID,
		AppID:             task.AppID,
		ProviderAccountID: execCtx.ProviderAccountID,
		Status:            "switch_provider",
		RequestData:       execCtx.RequestData,
		ResponseData:      execCtx.ResponseData,
		ErrorMessage:      fmt.Sprintf("switching provider, exclude current: %v", config.ExcludeCurrent),
	})

	// 重新推送到队列（选择器会自动选择其他供应商）
	go func() {
		// 稍微延迟一下再推送
		time.Sleep(1 * time.Second)
		if err := e.producer.Push(context.Background(), task); err != nil {
			e.logger.Error(fmt.Sprintf("failed to push task to queue for switch provider task_id=%s: %v", task.TaskID, err))
		}
	}()

	e.logger.Info(fmt.Sprintf("task scheduled for switch provider retry task_id=%s exclude_current=%v",
		task.TaskID, config.ExcludeCurrent))

	return &ExecuteResult{
		Action:      model.RuleActionSwitchProvider,
		ShouldRetry: true,
		RetryDelay:  1 * time.Second,
		TaskUpdated: true,
	}
}

// executeFail 执行失败动作
func (e *ActionExecutor) executeFail(ctx context.Context, result *EvaluateResult, execCtx *ExecuteContext) *ExecuteResult {
	task := execCtx.Task

	// 更新任务状态为失败
	task.Status = constants.TaskStatusFailed
	if err := e.taskDAO.Update(task); err != nil {
		e.logger.Error(fmt.Sprintf("failed to update task status to failed: %v", err))
	}

	// 记录失败日志
	if execCtx.ProviderAccountID > 0 {
		e.logDAO.Create(&model.PushLog{
			TaskID:            task.TaskID,
			AppID:             task.AppID,
			ProviderAccountID: execCtx.ProviderAccountID,
			Status:            "failed",
			RequestData:       execCtx.RequestData,
			ResponseData:      execCtx.ResponseData,
			ErrorMessage:      execCtx.ErrorMessage,
		})
	}

	ruleName := "default"
	if result.MatchedRule != nil {
		ruleName = result.MatchedRule.Name
	}

	e.logger.Info(fmt.Sprintf("task marked as failed task_id=%s rule=%s error=%s",
		task.TaskID, ruleName, execCtx.ErrorMessage))

	return &ExecuteResult{
		Action:       model.RuleActionFail,
		ShouldRetry:  false,
		TaskUpdated:  true,
		ErrorMessage: execCtx.ErrorMessage,
	}
}

// executeAlert 执行告警动作
func (e *ActionExecutor) executeAlert(ctx context.Context, result *EvaluateResult, execCtx *ExecuteContext) *ExecuteResult {
	task := execCtx.Task

	// 获取告警配置
	var config *model.AlertActionConfig
	var err error
	if result.MatchedRule != nil {
		config, err = result.MatchedRule.GetAlertConfig()
		if err != nil {
			e.logger.Error(fmt.Sprintf("failed to get alert config: %v", err))
			config = &model.AlertActionConfig{
				AlertLevel: "warning",
			}
		}
	} else {
		config = &model.AlertActionConfig{
			AlertLevel: "warning",
		}
	}

	// 发送告警
	alertSent := false
	if config.WebhookURL != "" {
		err := e.sendAlertWebhook(ctx, config, execCtx)
		if err != nil {
			e.logger.Error(fmt.Sprintf("failed to send alert webhook: %v", err))
		} else {
			alertSent = true
		}
	}

	// 记录告警日志
	e.logDAO.Create(&model.PushLog{
		TaskID:            task.TaskID,
		AppID:             task.AppID,
		ProviderAccountID: execCtx.ProviderAccountID,
		Status:            "alert",
		RequestData:       execCtx.RequestData,
		ResponseData:      execCtx.ResponseData,
		ErrorMessage:      fmt.Sprintf("alert sent: %v, level: %s, error: %s", alertSent, config.AlertLevel, execCtx.ErrorMessage),
	})

	// 告警后也标记任务为失败
	task.Status = constants.TaskStatusFailed
	if err := e.taskDAO.Update(task); err != nil {
		e.logger.Error(fmt.Sprintf("failed to update task status after alert: %v", err))
	}

	e.logger.Info(fmt.Sprintf("alert sent and task marked as failed task_id=%s alert_level=%s alert_sent=%v",
		task.TaskID, config.AlertLevel, alertSent))

	return &ExecuteResult{
		Action:       model.RuleActionAlert,
		ShouldRetry:  false,
		TaskUpdated:  true,
		AlertSent:    alertSent,
		ErrorMessage: execCtx.ErrorMessage,
	}
}

// sendAlertWebhook 发送告警 Webhook
func (e *ActionExecutor) sendAlertWebhook(ctx context.Context, config *model.AlertActionConfig, execCtx *ExecuteContext) error {
	if config.WebhookURL == "" {
		return nil
	}

	payload := map[string]interface{}{
		"alert_type":   "message_push_failure",
		"alert_level":  config.AlertLevel,
		"task_id":      execCtx.Task.TaskID,
		"app_id":       execCtx.Task.AppID,
		"receiver":     execCtx.Task.Receiver,
		"message_type": execCtx.Task.MessageType,
		"provider":     execCtx.ProviderCode,
		"error_code":   execCtx.ErrorCode,
		"error_msg":    execCtx.ErrorMessage,
		"timestamp":    time.Now().Unix(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal alert payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, config.WebhookURL, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create alert request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "MessagePush-Alert/1.0")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("alert webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// calculateBackoff 计算指数退避延迟
func (e *ActionExecutor) calculateBackoff(retryCount int, config *model.RetryActionConfig) time.Duration {
	delay := time.Duration(config.DelaySeconds) * time.Second
	for i := 0; i < retryCount; i++ {
		delay *= time.Duration(config.BackoffRate)
		maxDelay := time.Duration(config.MaxDelay) * time.Second
		if delay > maxDelay {
			return maxDelay
		}
	}
	return delay
}
