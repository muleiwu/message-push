package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/sender"
	internalHelper "cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
)

// CallbackService 回调服务
type CallbackService struct {
	logger         gsr.Logger
	taskDao        *dao.PushTaskDAO
	logDao         *dao.PushLogDAO
	callbackLogDao *dao.CallbackLogDAO
	senderFactory  *sender.Factory
	webhookService *WebhookService
	ruleEngine     *RuleEngineService
	actionExecutor *ActionExecutor
}

// NewCallbackService 创建回调服务
func NewCallbackService() *CallbackService {
	h := internalHelper.GetHelper()
	return &CallbackService{
		logger:         h.GetLogger(),
		taskDao:        dao.NewPushTaskDAO(),
		logDao:         dao.NewPushLogDAO(),
		callbackLogDao: dao.NewCallbackLogDAO(),
		senderFactory:  sender.NewFactory(),
		webhookService: NewWebhookService(),
		ruleEngine:     GetRuleEngineService(),
		actionExecutor: NewActionExecutor(),
	}
}

// HandleCallback 处理服务商回调
// 返回值：供应商期望的响应信息（实体，始终返回）
func (s *CallbackService) HandleCallback(ctx context.Context, providerCode string, req *sender.CallbackRequest) sender.CallbackResponse {
	// 默认响应（当无法获取处理器时使用）
	defaultResp := sender.CallbackResponse{
		StatusCode: 200,
		Body:       `{"code":0,"message":"ok"}`,
	}

	// 1. 获取对应的回调处理器
	handler, err := s.senderFactory.GetCallbackHandler(providerCode)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to get callback handler for provider=%s: %v", providerCode, err))
		return defaultResp
	}

	// 2. 解析回调数据（始终返回响应，即使有错误）
	resp, results, err := handler.HandleCallback(ctx, req)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to handle callback for provider=%s: %v", providerCode, err))
		// 返回供应商期望的响应，不再返回错误
		return resp
	}

	// 3. 更新任务状态
	rawData := buildRawDataJSON(req)
	for _, result := range results {
		if err := s.processCallbackResult(ctx, providerCode, result, rawData); err != nil {
			s.logger.Error(fmt.Sprintf("failed to process callback result provider_id=%s: %v", result.ProviderID, err))
			// 继续处理其他结果
		}
	}

	return resp
}

// processCallbackResult 处理单个回调结果
func (s *CallbackService) processCallbackResult(ctx context.Context, providerCode string, result *sender.CallbackResult, rawData string) error {
	// 1. 通过 ProviderID 在 push_logs 中查找日志，再关联任务
	pushLog, err := s.logDao.GetByProviderMsgID(result.ProviderID)
	if err != nil {
		// 如果找不到日志，可能是回调延迟或者日志已被清理
		// 仍然记录回调日志（无任务关联）
		s.callbackLogDao.Create(&model.CallbackLog{
			ProviderCode:   providerCode,
			ProviderID:     result.ProviderID,
			CallbackStatus: result.Status,
			ErrorCode:      result.ErrorCode,
			ErrorMessage:   result.ErrorMessage,
			RawData:        rawData,
		})
		s.logger.Warn(fmt.Sprintf("push_log not found for provider_id=%s", result.ProviderID))
		return nil
	}

	// 通过日志的 TaskID 查找任务
	task, err := s.taskDao.GetByTaskID(pushLog.TaskID)
	if err != nil {
		s.callbackLogDao.Create(&model.CallbackLog{
			ProviderCode:   providerCode,
			ProviderID:     result.ProviderID,
			CallbackStatus: result.Status,
			ErrorCode:      result.ErrorCode,
			ErrorMessage:   result.ErrorMessage,
			RawData:        rawData,
		})
		s.logger.Warn(fmt.Sprintf("task not found for task_id=%s (provider_id=%s)", pushLog.TaskID, result.ProviderID))
		return nil
	}

	// 2. 记录回调日志
	s.callbackLogDao.Create(&model.CallbackLog{
		TaskID:         task.TaskID,
		AppID:          task.AppID,
		ProviderCode:   providerCode,
		ProviderID:     result.ProviderID,
		CallbackStatus: result.Status,
		ErrorCode:      result.ErrorCode,
		ErrorMessage:   result.ErrorMessage,
		RawData:        rawData,
	})

	// 3. 更新任务状态
	oldStatus := task.Status

	// 设置回调状态和时间
	task.CallbackStatus = result.Status
	if !result.ReportTime.IsZero() {
		task.CallbackTime = &result.ReportTime
	} else {
		now := time.Now()
		task.CallbackTime = &now
	}

	switch result.Status {
	case "delivered":
		task.Status = constants.TaskStatusSuccess
		// 更新任务
		if err := s.taskDao.Update(task); err != nil {
			return fmt.Errorf("failed to update task: %w", err)
		}
	case "failed", "rejected":
		// 使用规则引擎评估回调失败
		evalReq := &EvaluateRequest{
			Scene:        model.RuleSceneCallbackFailure,
			ProviderCode: providerCode,
			MessageType:  task.MessageType,
			ErrorCode:    result.ErrorCode,
			ErrorMessage: result.ErrorMessage,
			Task:         task,
		}
		evalResult := s.ruleEngine.Evaluate(ctx, evalReq)

		// 构造执行上下文
		execCtx := &ExecuteContext{
			Task:              task,
			ProviderAccountID: 0, // 回调时可能没有供应商账号ID
			ProviderCode:      providerCode,
			ErrorCode:         result.ErrorCode,
			ErrorMessage:      result.ErrorMessage,
			RequestData:       rawData,
			ResponseData:      "",
		}

		// 执行规则动作
		execResult := s.actionExecutor.Execute(ctx, evalResult, execCtx)
		s.logger.Info(fmt.Sprintf("callback rule engine executed task_id=%s action=%s retry=%v",
			task.TaskID, execResult.Action, execResult.ShouldRetry))

		// 如果规则引擎决定重试，不更新状态为失败，等待重试
		if execResult.ShouldRetry {
			return nil
		}
		// 规则引擎已处理任务状态更新
	default:
		// 未知状态，记录回调状态但不更新任务主状态
		s.logger.Warn(fmt.Sprintf("unknown callback status=%s for task_id=%s", result.Status, task.TaskID))
		// 更新任务（回调状态已变化）
		if err := s.taskDao.Update(task); err != nil {
			return fmt.Errorf("failed to update task: %w", err)
		}
	}

	// 只有主状态发生变化时才触发 Webhook 通知
	if oldStatus != task.Status {
		s.logger.Info(fmt.Sprintf("task status updated task_id=%s old_status=%s new_status=%s",
			task.TaskID, oldStatus, task.Status))

		// 4. 触发业务方 Webhook 通知
		go func() {
			if err := s.webhookService.NotifyStatusChange(context.Background(), task, result); err != nil {
				s.logger.Error(fmt.Sprintf("failed to notify webhook for task_id=%s: %v", task.TaskID, err))
			}
		}()
	}

	return nil
}

// GetSupportedProviders 获取支持回调的服务商列表
func (s *CallbackService) GetSupportedProviders() []string {
	handlers := s.senderFactory.GetAllCallbackHandlers()
	providers := make([]string, 0, len(handlers))
	for _, handler := range handlers {
		providers = append(providers, handler.GetProviderCode())
	}
	return providers
}

// buildRawDataJSON 将回调请求数据转换为有效的 JSON 字符串
func buildRawDataJSON(req *sender.CallbackRequest) string {
	// 首先尝试解析为 JSON，如果已经是有效的 JSON 则直接使用
	if json.Valid(req.RawBody) {
		return string(req.RawBody)
	}
	// 否则构建结构化的 JSON 对象
	data := map[string]interface{}{
		"raw_body":     string(req.RawBody),
		"form_data":    req.FormData,
		"query_params": req.QueryParams,
	}
	jsonBytes, _ := json.Marshal(data)
	return string(jsonBytes)
}
