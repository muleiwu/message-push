package service

import (
	"context"
	"fmt"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/sender"
	internalHelper "cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
)

// CallbackService 回调服务
type CallbackService struct {
	logger         gsr.Logger
	taskDao        *dao.PushTaskDAO
	logDao         *dao.PushLogDAO
	senderFactory  *sender.Factory
	webhookService *WebhookService
}

// NewCallbackService 创建回调服务
func NewCallbackService() *CallbackService {
	h := internalHelper.GetHelper()
	return &CallbackService{
		logger:         h.GetLogger(),
		taskDao:        dao.NewPushTaskDAO(),
		logDao:         dao.NewPushLogDAO(),
		senderFactory:  sender.NewFactory(),
		webhookService: NewWebhookService(),
	}
}

// HandleCallback 处理服务商回调
func (s *CallbackService) HandleCallback(ctx context.Context, providerCode string, req *sender.CallbackRequest) error {
	// 1. 获取对应的回调处理器
	handler, err := s.senderFactory.GetCallbackHandler(providerCode)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to get callback handler for provider=%s: %v", providerCode, err))
		return fmt.Errorf("unsupported provider: %s", providerCode)
	}

	// 2. 解析回调数据
	results, err := handler.HandleCallback(ctx, req)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to handle callback for provider=%s: %v", providerCode, err))
		return fmt.Errorf("failed to parse callback data: %w", err)
	}

	// 3. 更新任务状态
	for _, result := range results {
		if err := s.processCallbackResult(ctx, providerCode, result); err != nil {
			s.logger.Error(fmt.Sprintf("failed to process callback result provider_id=%s: %v", result.ProviderID, err))
			// 继续处理其他结果
		}
	}

	return nil
}

// processCallbackResult 处理单个回调结果
func (s *CallbackService) processCallbackResult(ctx context.Context, providerCode string, result *sender.CallbackResult) error {
	// 1. 通过 ProviderID 查找任务
	task, err := s.taskDao.GetByProviderID(result.ProviderID)
	if err != nil {
		// 如果找不到任务，可能是回调延迟或者任务已被清理
		s.logger.Warn(fmt.Sprintf("task not found for provider_id=%s", result.ProviderID))
		return nil
	}

	// 2. 更新任务状态
	oldStatus := task.Status
	switch result.Status {
	case "delivered":
		task.Status = constants.TaskStatusSuccess
	case "failed", "rejected":
		task.Status = constants.TaskStatusFailed
	default:
		// 未知状态，记录日志但不更新
		s.logger.Warn(fmt.Sprintf("unknown callback status=%s for task_id=%s", result.Status, task.TaskID))
		return nil
	}

	// 只有状态发生变化时才更新
	if oldStatus != task.Status {
		if err := s.taskDao.Update(task); err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		s.logger.Info(fmt.Sprintf("task status updated task_id=%s old_status=%s new_status=%s",
			task.TaskID, oldStatus, task.Status))

		// 3. 触发业务方 Webhook 通知
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
