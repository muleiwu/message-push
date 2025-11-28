package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/queue"
	"cnb.cool/mliev/push/message-push/app/selector"
	"cnb.cool/mliev/push/message-push/app/sender"
	"cnb.cool/mliev/push/message-push/app/service"
	internalHelper "cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
)

// MessageHandler 消息处理器
type MessageHandler struct {
	logger              gsr.Logger
	taskDao             *dao.PushTaskDAO
	logDao              *dao.PushLogDAO
	selector            *selector.ChannelSelector
	senderFactory       *sender.Factory
	retryHelper         *helper.RetryHelper
	signatureMappingDao *dao.ChannelSignatureMappingDAO
	templateHelper      *helper.TemplateHelper
	ruleEngine          *service.RuleEngineService
	actionExecutor      *service.ActionExecutor
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler() *MessageHandler {
	return &MessageHandler{
		logger:              internalHelper.GetHelper().GetLogger(),
		taskDao:             dao.NewPushTaskDAO(),
		logDao:              dao.NewPushLogDAO(),
		selector:            selector.NewChannelSelector(),
		senderFactory:       sender.NewFactory(),
		retryHelper:         helper.NewRetryHelper(),
		signatureMappingDao: dao.NewChannelSignatureMappingDAO(internalHelper.GetHelper().GetDatabase()),
		templateHelper:      helper.NewTemplateHelper(),
		ruleEngine:          service.GetRuleEngineService(),
		actionExecutor:      service.NewActionExecutor(),
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
		h.handleEarlyFailure(task, 0, err.Error())
		return err
	}

	// 从节点直接获取服务商账号信息
	providerAccount := node.ProviderAccount
	if providerAccount == nil {
		err := fmt.Errorf("provider account not found in channel node")
		h.logger.Error(fmt.Sprintf("failed to get provider account task_id=%s: %v", taskID, err))
		h.handleEarlyFailure(task, 0, err.Error())
		return err
	}

	// 获取发送器（按服务商代码获取）
	messageSender, err := h.senderFactory.GetSender(providerAccount.ProviderCode)
	if err != nil {
		h.logger.Error(fmt.Sprintf("failed to get sender task_id=%s: %v", taskID, err))
		h.handleEarlyFailure(task, providerAccount.ID, err.Error())
		return err
	}

	// 查找签名映射，直接获取供应商签名
	var providerSignature *model.ProviderSignature
	if task.Signature != "" {
		providerSignature, err = h.signatureMappingDao.GetByChannelIDAndSignatureName(task.ChannelID, task.Signature, providerAccount.ID)
		if err != nil {
			h.logger.Warn(fmt.Sprintf("signature mapping not found task_id=%s signature=%s: %v", taskID, task.Signature, err))
		} else if providerSignature != nil {
			h.logger.Info(fmt.Sprintf("signature resolved task_id=%s signature_name=%s signature_code=%s", taskID, task.Signature, providerSignature.SignatureCode))
		}
	}

	// 解析模板参数并进行映射转换
	var mappedParams map[string]string
	if task.TemplateParams != "" && node.ChannelTemplateBinding != nil {
		// 解析任务的模板参数
		var templateParams map[string]string
		if err := json.Unmarshal([]byte(task.TemplateParams), &templateParams); err != nil {
			h.logger.Warn(fmt.Sprintf("failed to parse template params task_id=%s: %v", taskID, err))
		} else {
			// 获取参数映射配置
			paramMapping, err := node.ChannelTemplateBinding.GetParamMapping()
			if err != nil {
				h.logger.Warn(fmt.Sprintf("failed to get param mapping task_id=%s: %v", taskID, err))
			} else if len(paramMapping) > 0 {
				// 执行参数映射转换
				mappedParams = h.templateHelper.MapParams(templateParams, paramMapping)
				h.logger.Info(fmt.Sprintf("params mapped task_id=%s original=%v mapped=%v", taskID, templateParams, mappedParams))
			}
		}
	}

	// 发送消息
	sendReq := &sender.SendRequest{
		Task:                   task,
		ProviderAccount:        providerAccount,
		ChannelTemplateBinding: node.ChannelTemplateBinding,
		Signature:              providerSignature,
		MappedParams:           mappedParams,
	}

	resp, err := messageSender.Send(ctx, sendReq)
	if err != nil {
		h.logger.Error(fmt.Sprintf("sender error task_id=%s: %v", taskID, err))
		// 如果 Send 返回了 resp（即使有 error），使用它来记录日志
		if resp != nil {
			h.handleSendError(task, providerAccount.ID, resp)
		} else {
			h.handleEarlyFailure(task, providerAccount.ID, err.Error())
		}
		return err
	}

	// 处理发送结果
	if resp.Success {
		h.handleSuccess(task, providerAccount.ID, resp)
	} else {
		h.handleSendError(task, providerAccount.ID, resp)
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

	// 传入 appID 和 receiver 用于 5 分钟内同一接收者切换供应商策略
	node, err := h.selector.Select(ctx, channelID, task.MessageType, task.AppID, task.Receiver)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// handleSuccess 处理成功
func (h *MessageHandler) handleSuccess(task *model.PushTask, providerAccountID uint, resp *sender.SendResponse) {
	task.Status = resp.Status // 使用发送器返回的状态（processing=等待回调, success=直接成功）
	h.taskDao.Update(task)

	// 记录日志（每次新增，便于观测请求链路），ProviderMsgID 保存在日志中用于回调匹配
	h.logDao.Create(&model.PushLog{
		TaskID:            task.TaskID,
		AppID:             task.AppID,
		ProviderAccountID: providerAccountID,
		ProviderMsgID:     resp.ProviderID,
		Status:            "success",
		RequestData:       resp.RequestData,
		ResponseData:      resp.ResponseData,
	})

	// 通知选择器成功
	h.selector.ReportSuccess(providerAccountID)

	h.logger.Info(fmt.Sprintf("message sent successfully task_id=%s provider_id=%s status=%s", task.TaskID, resp.ProviderID, resp.Status))
}

// handleSendError 处理发送错误（使用规则引擎）
func (h *MessageHandler) handleSendError(task *model.PushTask, providerAccountID uint, resp *sender.SendResponse) {
	// 通知选择器失败
	h.selector.ReportFailure(providerAccountID)

	// 获取供应商代码
	providerCode := ""
	providerAccountDao := dao.NewProviderAccountDAO()
	if account, err := providerAccountDao.GetByID(providerAccountID); err == nil {
		providerCode = account.ProviderCode
	}

	// 使用规则引擎评估
	evalReq := &service.EvaluateRequest{
		Scene:        model.RuleSceneSendFailure,
		ProviderCode: providerCode,
		MessageType:  task.MessageType,
		ErrorCode:    resp.ErrorCode,
		ErrorMessage: resp.ErrorMessage,
		Task:         task,
	}
	evalResult := h.ruleEngine.Evaluate(context.Background(), evalReq)

	// 构造执行上下文
	execCtx := &service.ExecuteContext{
		Task:              task,
		ProviderAccountID: providerAccountID,
		ProviderCode:      providerCode,
		ErrorCode:         resp.ErrorCode,
		ErrorMessage:      resp.ErrorMessage,
		RequestData:       resp.RequestData,
		ResponseData:      resp.ResponseData,
	}

	// 执行规则动作
	execResult := h.actionExecutor.Execute(context.Background(), evalResult, execCtx)

	h.logger.Info(fmt.Sprintf("rule engine executed task_id=%s action=%s retry=%v",
		task.TaskID, execResult.Action, execResult.ShouldRetry))
}

// handleEarlyFailure 处理早期失败（发送前的错误，无供应商响应数据）
// 早期失败不使用规则引擎，直接标记失败
func (h *MessageHandler) handleEarlyFailure(task *model.PushTask, providerAccountID uint, errorMsg string) {
	task.Status = constants.TaskStatusFailed
	h.taskDao.Update(task)

	// 记录日志（每次新增，便于观测请求链路）
	if providerAccountID > 0 {
		h.logDao.Create(&model.PushLog{
			TaskID:            task.TaskID,
			AppID:             task.AppID,
			ProviderAccountID: providerAccountID,
			Status:            "failed",
			RequestData:       "{}",
			ResponseData:      "{}",
			ErrorMessage:      errorMsg,
		})
	}

	h.logger.Error(fmt.Sprintf("message failed task_id=%s error=%s", task.TaskID, errorMsg))
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
