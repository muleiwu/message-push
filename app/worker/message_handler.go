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
	logger             gsr.Logger
	taskDao            *dao.PushTaskDAO
	providerChannelDao *dao.ProviderChannelDAO
	providerAccountDao *dao.ProviderAccountDAO
	logDao             *dao.PushLogDAO
	selector           *selector.ChannelSelector
	senderFactory      *sender.Factory
	retryHelper        *helper.RetryHelper
	templateHelper     *helper.TemplateHelper
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler() *MessageHandler {
	return &MessageHandler{
		logger:             internalHelper.GetHelper().GetLogger(),
		taskDao:            dao.NewPushTaskDAO(),
		providerChannelDao: dao.NewProviderChannelDAO(),
		providerAccountDao: dao.NewProviderAccountDAO(),
		logDao:             dao.NewPushLogDAO(),
		selector:           selector.NewChannelSelector(),
		senderFactory:      sender.NewFactory(),
		retryHelper:        helper.NewRetryHelper(),
		templateHelper:     helper.NewTemplateHelper(),
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
		h.handleFailure(task, err.Error())
		return err
	}

	// 获取服务商账号信息
	providerAccount, err := h.providerAccountDao.GetByID(node.Channel.ProviderID)
	if err != nil {
		h.logger.Error(fmt.Sprintf("failed to get provider account task_id=%s: %v", taskID, err))
		h.handleFailure(task, err.Error())
		return err
	}

	// 转换为旧的Provider结构（为了兼容现有Sender接口）
	provider := &model.Provider{
		ID:           providerAccount.ID,
		ProviderCode: providerAccount.AccountCode,
		ProviderName: providerAccount.AccountName,
		ProviderType: providerAccount.ProviderType,
		Config:       providerAccount.Config,
		Status:       providerAccount.Status,
	}

	// 尝试使用新的模板系统处理模板和参数
	h.processTemplateBinding(task, node)

	// 获取发送器
	messageSender, err := h.senderFactory.GetSender(task.MessageType)
	if err != nil {
		h.logger.Error(fmt.Sprintf("failed to get sender task_id=%s: %v", taskID, err))
		h.handleFailure(task, err.Error())
		return err
	}

	// 发送消息
	sendReq := &sender.SendRequest{
		Task:            task,
		ProviderChannel: node.Channel,
		Provider:        provider,
		Relation:        node.Relation,
	}

	resp, err := messageSender.Send(ctx, sendReq)
	if err != nil {
		h.logger.Error(fmt.Sprintf("sender error task_id=%s: %v", taskID, err))
		h.handleSendError(task, node.Channel, err.Error())
		return err
	}

	// 处理发送结果
	if resp.Success {
		h.handleSuccess(task, node.Channel, resp.ProviderID)
	} else {
		h.handleSendError(task, node.Channel, resp.ErrorMessage)
	}

	return nil
}

// selectChannel 选择发送通道
func (h *MessageHandler) selectChannel(ctx context.Context, task *model.PushTask) (*selector.ChannelNode, error) {
	// 如果任务已指定服务商通道，直接使用 (需要反查 Relation，这里简化处理：如果是指定通道，可能没有Relation配置)
	// 这种情况通常是测试或手动指定。如果必须Relation，则无法直接指定ProviderChannelID。
	// 假设必须走Selector流程，或者任务中携带了必要信息。
	// 暂时不支持直接指定ProviderChannelID bypass Selector logic completely regarding Relation.
	// 或者我们可以查一下Relation。
	// 为了兼容，如果指定了ProviderChannelID，我们构造一个Dummy Node
	if task.ProviderChannelID != nil {
		pc, err := h.providerChannelDao.GetByID(*task.ProviderChannelID)
		if err != nil {
			return nil, err
		}
		// 尝试找Relation，如果找不到就用空Relation
		// 这里省略复杂逻辑，假设指定通道时不需要Relation特定配置（签名等可能在Provider或Task中）
		return &selector.ChannelNode{
			Channel:  pc,
			Relation: &model.ChannelProviderRelation{
				// 默认值
			},
		}, nil
	}

	// 使用选择器选择通道
	channelID, err := parseUint(task.PushChannelID)
	if err != nil {
		return nil, fmt.Errorf("invalid push_channel_id: %w", err)
	}

	node, err := h.selector.Select(ctx, channelID, task.MessageType)
	if err != nil {
		return nil, err
	}

	// 更新任务的服务商通道ID
	task.ProviderChannelID = &node.Channel.ID
	h.taskDao.Update(task)

	return node, nil
}

// handleSuccess 处理成功
func (h *MessageHandler) handleSuccess(task *model.PushTask, providerChannel *model.ProviderChannel, providerID string) {
	task.Status = constants.TaskStatusSuccess
	h.taskDao.Update(task)

	// 记录日志
	h.logDao.Create(&model.PushLog{
		TaskID:            task.TaskID,
		AppID:             task.AppID,
		ProviderChannelID: providerChannel.ID,
		Status:            "success",
		ResponseData:      fmt.Sprintf("{\"provider_id\":\"%s\"}", providerID),
	})

	// 通知选择器成功
	h.selector.ReportSuccess(providerChannel.ProviderID, providerChannel.ID)

	h.logger.Info(fmt.Sprintf("message sent successfully task_id=%s provider_id=%s", task.TaskID, providerID))
}

// handleSendError 处理发送错误
func (h *MessageHandler) handleSendError(task *model.PushTask, providerChannel *model.ProviderChannel, errorMsg string) {
	// 通知选择器失败
	h.selector.ReportFailure(providerChannel.ProviderID, providerChannel.ID)

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
		h.handleFailure(task, errorMsg)
	}
}

// handleFailure 处理失败
func (h *MessageHandler) handleFailure(task *model.PushTask, errorMsg string) {
	task.Status = constants.TaskStatusFailed
	h.taskDao.Update(task)

	// 记录日志
	if task.ProviderChannelID != nil {
		h.logDao.Create(&model.PushLog{
			TaskID:            task.TaskID,
			AppID:             task.AppID,
			ProviderChannelID: *task.ProviderChannelID,
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
