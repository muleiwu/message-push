package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/queue"
	"cnb.cool/mliev/push/message-push/app/selector"
	"cnb.cool/mliev/push/message-push/app/sender"
	internalHelper "cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
)

// MessageHandler 消息处理器
type MessageHandler struct {
	logger         gsr.Logger
	taskDao        *dao.PushTaskDAO
	logDao         *dao.PushLogDAO
	selector       *selector.ChannelSelector
	senderFactory  *sender.Factory
	retryHelper    *helper.RetryHelper
	templateHelper *helper.TemplateHelper
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler() *MessageHandler {
	return &MessageHandler{
		logger:         internalHelper.GetHelper().GetLogger(),
		taskDao:        dao.NewPushTaskDAO(),
		logDao:         dao.NewPushLogDAO(),
		selector:       selector.NewChannelSelector(),
		senderFactory:  sender.NewFactory(),
		retryHelper:    helper.NewRetryHelper(),
		templateHelper: helper.NewTemplateHelper(),
	}
}

// Handle 处理消息
func (h *MessageHandler) Handle(ctx context.Context, msg *queue.Message) error {
	// 解析任务ID
	taskID, ok := msg.Data["task_id"].(string)
	if !ok {
		return fmt.Errorf("invalid task_id in message")
	}

	// 获取任务
	task, err := h.taskDao.GetByTaskID(taskID)
	if err != nil {
		h.logger.Error(fmt.Sprintf("failed to get task id=%s: %v", taskID, err))
		return err
	}

	// 更新任务状态为处理中
	task.Status = constants.TaskStatusProcessing
	h.taskDao.Update(task)

	// 选择通道
	node, err := h.selectChannel(ctx, task)
	if err != nil {
		h.logger.Error(fmt.Sprintf("failed to select channel task_id=%s: %v", taskID, err))
		h.handleFailure(task, 0, err.Error())
		return err
	}

	// 从节点直接获取服务商账号信息
	providerAccount := node.ProviderAccount
	if providerAccount == nil {
		err := fmt.Errorf("provider account not found in channel node")
		h.logger.Error(fmt.Sprintf("failed to get provider account task_id=%s: %v", taskID, err))
		h.handleFailure(task, 0, err.Error())
		return err
	}

	// 更新任务的服务商账号ID
	task.ProviderAccountID = &providerAccount.ID
	h.taskDao.Update(task)

	// 尝试使用新的模板系统处理模板和参数
	h.processTemplateBinding(task, node)

	// 获取发送器（按服务商代码获取）
	messageSender, err := h.senderFactory.GetSender(providerAccount.ProviderCode)
	if err != nil {
		h.logger.Error(fmt.Sprintf("failed to get sender task_id=%s: %v", taskID, err))
		h.handleFailure(task, providerAccount.ID, err.Error())
		return err
	}

	// 发送消息（不自动加载签名，由通道配置或API调用指定）
	sendReq := &sender.SendRequest{
		Task:            task,
		ProviderAccount: providerAccount,
		Relation:        node.Relation,
		Signature:       nil, // 签名由通道配置或API调用指定，这里不自动加载
	}

	resp, err := messageSender.Send(ctx, sendReq)
	if err != nil {
		h.logger.Error(fmt.Sprintf("sender error task_id=%s: %v", taskID, err))
		h.handleSendError(task, providerAccount.ID, err.Error())
		return err
	}

	// 处理发送结果
	if resp.Success {
		h.handleSuccess(task, providerAccount.ID, resp.ProviderID)
	} else {
		h.handleSendError(task, providerAccount.ID, resp.ErrorMessage)
	}

	return nil
}

// selectChannel 选择发送通道
func (h *MessageHandler) selectChannel(ctx context.Context, task *model.PushTask) (*selector.ChannelNode, error) {
	// 使用选择器选择通道
	channelID, err := parseUint(task.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("invalid channel_id: %w", err)
	}

	node, err := h.selector.Select(ctx, channelID, task.MessageType)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// handleSuccess 处理成功
func (h *MessageHandler) handleSuccess(task *model.PushTask, providerAccountID uint, providerID string) {
	task.Status = constants.TaskStatusSuccess
	task.ProviderMsgID = providerID // 保存服务商消息ID，用于回调匹配
	h.taskDao.Update(task)

	// 记录日志
	h.logDao.Create(&model.PushLog{
		TaskID:            task.TaskID,
		AppID:             task.AppID,
		ProviderAccountID: providerAccountID,
		Status:            "success",
		ResponseData:      fmt.Sprintf("{\"provider_id\":\"%s\"}", providerID),
	})

	// 通知选择器成功
	h.selector.ReportSuccess(providerAccountID)

	h.logger.Info(fmt.Sprintf("message sent successfully task_id=%s provider_id=%s", task.TaskID, providerID))
}

// handleSendError 处理发送错误
func (h *MessageHandler) handleSendError(task *model.PushTask, providerAccountID uint, errorMsg string) {
	// 通知选择器失败
	h.selector.ReportFailure(providerAccountID)

	// 判断是否需要重试
	shouldRetry, delay := h.retryHelper.ShouldRetry(errorMsg, task.RetryCount, task.MaxRetry)

	if shouldRetry {
		task.RetryCount++
		h.taskDao.Update(task)

		// 重新推送到队列
		go func() {
			time.Sleep(delay)
			producer := queue.NewProducer(internalHelper.GetHelper().GetRedis())
			producer.Push(context.Background(), task)
		}()

		h.logger.Info(fmt.Sprintf("message will retry task_id=%s retry_count=%d delay=%v", task.TaskID, task.RetryCount, delay))
	} else {
		h.handleFailure(task, providerAccountID, errorMsg)
	}
}

// handleFailure 处理失败
func (h *MessageHandler) handleFailure(task *model.PushTask, providerAccountID uint, errorMsg string) {
	task.Status = constants.TaskStatusFailed
	h.taskDao.Update(task)

	// 记录日志
	if providerAccountID > 0 {
		h.logDao.Create(&model.PushLog{
			TaskID:            task.TaskID,
			AppID:             task.AppID,
			ProviderAccountID: providerAccountID,
			Status:            "failed",
			ErrorMessage:      errorMsg,
		})
	}

	h.logger.Error(fmt.Sprintf("message failed task_id=%s error=%s", task.TaskID, errorMsg))
}

// processTemplateBinding 处理模板绑定和参数映射（使用新的ChannelTemplateBinding）
func (h *MessageHandler) processTemplateBinding(task *model.PushTask, node *selector.ChannelNode) {
	// 如果任务没有模板代码，跳过
	if task.TemplateCode == "" {
		return
	}

	// 如果没有通道模板绑定配置，使用原有逻辑（向后兼容）
	if node.ChannelTemplateBinding == nil {
		h.logger.Info(fmt.Sprintf("no channel template binding found for task_id=%s, using legacy mode", task.TaskID))
		return
	}

	binding := node.ChannelTemplateBinding

	// 如果有供应商模板配置，使用供应商模板
	if binding.ProviderTemplate != nil {
		h.logger.Info(fmt.Sprintf("using channel template binding: provider_template=%s", binding.ProviderTemplate.TemplateCode))

		// 更新任务的模板代码为供应商模板代码
		task.TemplateCode = binding.ProviderTemplate.TemplateCode

		// 解析原始参数
		var originalParams map[string]interface{}
		if task.TemplateParams != "" {
			if err := json.Unmarshal([]byte(task.TemplateParams), &originalParams); err != nil {
				h.logger.Error(fmt.Sprintf("failed to parse template params: %v", err))
				return
			}
		} else {
			originalParams = make(map[string]interface{})
		}

		// 获取参数映射
		paramMapping, err := binding.GetParamMapping()
		if err != nil {
			h.logger.Error(fmt.Sprintf("failed to get param mapping: %v", err))
			return
		}

		// 如果有参数映射，转换参数
		if len(paramMapping) > 0 {
			mappedParams := h.templateHelper.MapParams(originalParams, paramMapping)

			// 更新任务的模板参数
			if mappedJSON, err := json.Marshal(mappedParams); err == nil {
				task.TemplateParams = string(mappedJSON)
				h.logger.Info(fmt.Sprintf("params mapped from %v to %v", originalParams, mappedParams))
			}
		}

		// 如果需要渲染内容（根据系统模板内容）
		// 注意：binding 中没有直接关联 MessageTemplate，如果需要可以通过 Channel 获取
		// 这里暂时跳过内容渲染，或者可以根据需要扩展
	}
}

// parseUint 解析uint
func parseUint(value interface{}) (uint, error) {
	switch v := value.(type) {
	case uint:
		return v, nil
	case uint64:
		return uint(v), nil
	case int:
		return uint(v), nil
	case int64:
		return uint(v), nil
	case string:
		i, err := strconv.ParseUint(v, 10, 32)
		return uint(i), err
	case json.Number:
		i, err := v.Int64()
		return uint(i), err
	default:
		return 0, fmt.Errorf("unsupported type: %T", value)
	}
}
