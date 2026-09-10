package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	internalHelper "cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/readiness"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/timeutil"
	"cnb.cool/mliev/push/message-push/modules/channel"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/queue"
	"cnb.cool/mliev/push/message-push/modules/ruleengine"
	"cnb.cool/mliev/push/message-push/modules/sender"
	registry "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"cnb.cool/mliev/push/message-push/modules/template"
	"github.com/muleiwu/gsr"
)

// sanitizeJSONData 确保 JSON 数据有效，空字符串转换为 "{}"
func sanitizeJSONData(data string) string {
	if data == "" {
		return "{}"
	}
	return data
}

// MessageHandler 消息处理器
type MessageHandler struct {
	logger              gsr.Logger
	taskDao             *dao.PushTaskDAO
	attachmentDao       *dao.EmailAttachmentDAO
	logDao              *dao.PushLogDAO
	selector            channel.Selector
	senderResolver      sender.Resolver
	retryHelper         *helper.RetryHelper
	signatureMappingDao *dao.ChannelSignatureMappingDAO
	templateHelper      template.Renderer
	loadBinding         func(uint) (*model.ChannelTemplateBinding, error)
	ruleEngine          ruleengine.Engine
	actionExecutor      *service.ActionExecutor
	terminalService     *service.TaskTerminalService
}

type deliverySignatureLookup interface {
	GetByChannelIDAndSignatureName(channelID uint, signatureName string, providerID uint) (*model.ProviderSignature, error)
}

type deliverySenderLookup interface {
	GetSender(providerCode string) (sender.Sender, error)
}

// NewMessageHandler 创建消息处理器
func NewMessageHandler() *MessageHandler {
	return &MessageHandler{
		logger:              internalHelper.GetLogger(),
		taskDao:             dao.NewPushTaskDAO(),
		attachmentDao:       dao.NewEmailAttachmentDAO(),
		logDao:              dao.NewPushLogDAO(),
		selector:            channel.GetSelector(),
		senderResolver:      sender.GetResolver(),
		retryHelper:         helper.NewRetryHelper(),
		signatureMappingDao: dao.NewChannelSignatureMappingDAO(internalHelper.GetDatabase()),
		templateHelper:      template.GetRenderer(),
		loadBinding:         dao.NewChannelTemplateBindingDAO().GetByID,
		ruleEngine:          ruleengine.GetEngine(),
		actionExecutor:      service.NewActionExecutor(),
		terminalService:     service.NewTaskTerminalService(),
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

	// CAS 抢占（pending→processing）：队列 at-least-once 语义下重复消息抢占失败，直接 Ack 跳过
	claimed, err := h.taskDao.ClaimForProcessing(taskID)
	if err != nil {
		h.logger.Error(fmt.Sprintf("failed to claim task id=%s: %v", taskID, err))
		return err
	}
	if !claimed {
		h.logger.Info(fmt.Sprintf("skip duplicate message: task not in pending status task_id=%s status=%s", taskID, task.Status))
		return nil
	}
	task.Status = constants.TaskStatusProcessing
	snapshot := newSendSnapshot(task)

	// 选择通道
	node, err := h.selectChannel(ctx, task)
	if err != nil {
		h.logger.Error(fmt.Sprintf("failed to select channel task_id=%s: %v", taskID, err))
		h.handleEarlyFailure(task, 0, err.Error(), snapshot)
		return err
	}

	// 从节点直接获取服务商账号信息
	providerAccount := node.ProviderAccount
	if providerAccount == nil {
		err := fmt.Errorf("provider account not found in channel node")
		h.logger.Error(fmt.Sprintf("failed to get provider account task_id=%s: %v", taskID, err))
		h.handleEarlyFailure(task, 0, err.Error(), snapshot)
		return err
	}
	snapshot.ProviderAccountID = providerAccount.ID
	snapshot.ProviderName = providerAccount.AccountName
	snapshot.ProviderCode = providerAccount.ProviderCode
	if err := h.persistSelectedProvider(task, providerAccount.ID); err != nil {
		h.logger.Error(fmt.Sprintf("failed to persist selected provider account task_id=%s provider_id=%d: %v", taskID, providerAccount.ID, err))
		return err
	}

	providerMeta, err := registry.GetByCode(providerAccount.ProviderCode)
	if err != nil {
		err = fmt.Errorf("provider is not registered: %w", err)
		h.logger.Error(fmt.Sprintf("failed to resolve provider metadata task_id=%s: %v", taskID, err))
		h.handleEarlyFailure(task, providerAccount.ID, err.Error(), snapshot)
		return err
	}

	// Resolve required signatures before touching the sender. This keeps queued
	// tasks fail-closed when mappings are removed or disabled after acceptance.
	providerSignature, messageSender, err := resolveDeliveryDependencies(
		h.signatureMappingDao,
		h.senderResolver,
		task,
		providerAccount,
		providerMeta.RequiresSignature,
	)
	if err != nil {
		h.logger.Error(fmt.Sprintf("failed to resolve delivery dependencies task_id=%s provider_id=%d: %v", taskID, providerAccount.ID, err))
		h.handleEarlyFailure(task, providerAccount.ID, err.Error(), snapshot)
		return err
	}
	if providerSignature != nil {
		snapshot.SignatureValue = providerSignature.SignatureCode
		h.logger.Info(fmt.Sprintf("signature resolved task_id=%s signature_name=%s signature_code=%s", taskID, task.Signature, providerSignature.SignatureCode))
	}

	var attachments []*model.EmailAttachment
	if task.AttachmentGroupID != "" {
		attachmentDao := h.attachmentDao
		if attachmentDao == nil {
			attachmentDao = dao.NewEmailAttachmentDAO()
		}
		attachments, err = attachmentDao.GetForSend(task.AttachmentGroupID)
		if err != nil {
			h.logger.Error(fmt.Sprintf("failed to load email attachments task_id=%s provider_id=%d: %v", taskID, providerAccount.ID, err))
			h.handleEarlyFailure(task, providerAccount.ID, err.Error(), snapshot)
			return err
		}
	}

	if task.MessageType == constants.MessageTypeSMS {
		if node.ChannelTemplateBinding == nil {
			err := fmt.Errorf("短信模板绑定不存在")
			h.handleEarlyFailure(task, providerAccount.ID, err.Error(), snapshot)
			return err
		}
		load := h.loadBinding
		if load == nil {
			load = dao.NewChannelTemplateBindingDAO().GetByID
		}
		fresh, err := load(node.ChannelTemplateBinding.ID)
		if err == nil && (fresh == nil || fresh.ChannelID != task.ChannelID || fresh.Channel == nil || fresh.Channel.Type != task.MessageType || fresh.Channel.Status != 1 || fresh.Channel.MessageTemplate == nil || fresh.Channel.MessageTemplate.Status != 1 || fresh.ProviderID != providerAccount.ID) {
			err = fmt.Errorf("短信通道配置已变化")
		}
		if err == nil {
			if fresh.ProviderTemplate == nil || fresh.ProviderTemplate.ProviderAccount == nil || fresh.ProviderTemplate.ProviderAccount.ProviderCode != providerMeta.Code {
				err = fmt.Errorf("短信供应商配置已变化")
			}
		}
		if err == nil {
			variables, variableErr := fresh.Channel.MessageTemplate.GetVariables()
			if variableErr != nil || len(readiness.ValidateBinding(task.MessageType, variables, fresh)) != 0 {
				err = fmt.Errorf("短信模板或参数映射不可用，请重新确认")
			}
		}
		if err != nil {
			h.handleEarlyFailure(task, providerAccount.ID, err.Error(), snapshot)
			return err
		}
		node.ChannelTemplateBinding = fresh
		providerAccount = fresh.ProviderTemplate.ProviderAccount
	}
	snapshot.MessageContent = template.PrepareContent(h.templateHelper, task.TemplateParams, node.ChannelTemplateBinding)
	snapshot.CapturedAt = timeutil.FormatRFC3339(timeutil.Now())
	if snapshot.UnavailableReason != "" {
		if task.MessageType == constants.MessageTypeSMS {
			err := fmt.Errorf("%s", snapshot.UnavailableReason)
			h.handleEarlyFailure(task, providerAccount.ID, err.Error(), snapshot)
			return err
		}
		h.logger.Warn(fmt.Sprintf("message content unavailable task_id=%s: %s", taskID, snapshot.UnavailableReason))
	}

	// 发送消息
	sendReq := &sender.SendRequest{
		Task:                   task,
		ProviderAccount:        providerAccount,
		ChannelTemplateBinding: node.ChannelTemplateBinding,
		Signature:              providerSignature,
		MappedParams:           snapshot.MappedParams,
		RenderedContent:        snapshot.Content,
		Attachments:            attachments,
	}

	// 统一解析手机号，供发送器按地区直接判断（仅 SMS 类型）
	if task.MessageType == constants.MessageTypeSMS {
		if phone := helper.ParsePhoneNumber(task.Receiver); phone.Valid {
			sendReq.PhoneRegion = phone.Region
			sendReq.PhoneCountryCode = phone.CountryCode
			sendReq.PhoneNationalNumber = phone.NationalNumber
			sendReq.PhoneE164 = phone.E164
		}
	}

	resp, err := messageSender.Send(ctx, sendReq)
	if err != nil {
		h.logger.Error(fmt.Sprintf("sender error task_id=%s: %v", taskID, err))
		// 如果 Send 返回了 resp（即使有 error），使用它来记录日志
		if resp != nil {
			if terminalErr := h.handleSendError(task, providerAccount.ID, resp, snapshot); terminalErr != nil {
				h.logger.Error(fmt.Sprintf("failed to persist sender failure task_id=%s: %v", taskID, terminalErr))
			}
		} else {
			h.handleEarlyFailure(task, providerAccount.ID, err.Error(), snapshot)
		}
		return err
	}

	if resp == nil {
		err := fmt.Errorf("sender returned no response")
		h.handleEarlyFailure(task, providerAccount.ID, err.Error(), snapshot)
		return err
	}

	// 处理发送结果
	if resp.Success {
		return h.handleSuccess(task, providerAccount.ID, resp, snapshot)
	}
	return h.handleSendError(task, providerAccount.ID, resp, snapshot)
}

func resolveDeliveryDependencies(
	signatures deliverySignatureLookup,
	senders deliverySenderLookup,
	task *model.PushTask,
	account *model.ProviderAccount,
	requiresSignature bool,
) (*model.ProviderSignature, sender.Sender, error) {
	if task == nil || account == nil {
		return nil, nil, fmt.Errorf("task or provider account is missing")
	}

	alias := strings.TrimSpace(task.Signature)
	var providerSignature *model.ProviderSignature
	if alias != "" {
		resolved, err := signatures.GetByChannelIDAndSignatureName(task.ChannelID, alias, account.ID)
		if err != nil {
			if requiresSignature {
				return nil, nil, fmt.Errorf("required signature alias cannot be resolved: %w", err)
			}
		} else {
			providerSignature = resolved
		}
	}
	if requiresSignature && providerSignature == nil {
		return nil, nil, fmt.Errorf("required signature is missing")
	}

	messageSender, err := senders.GetSender(account.ProviderCode)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get sender: %w", err)
	}
	return providerSignature, messageSender, nil
}

// selectChannel 选择发送通道
func (h *MessageHandler) selectChannel(ctx context.Context, task *model.PushTask) (*channel.ChannelNode, error) {
	// 使用选择器选择通道
	channelID, err := parseUint(task.ChannelID)
	if err != nil {
		return nil, fmt.Errorf("invalid channel_id: %w", err)
	}

	// 获取需要排除的供应商ID列表（规则引擎切换供应商时设置）
	excludeProviderIDs := task.GetExcludeProviderIDs()

	// 传入 appID 和 receiver 用于 5 分钟内同一接收者切换供应商策略
	// 同时传入排除列表用于规则引擎的切换供应商功能
	node, err := h.selector.SelectWithExcludes(ctx, channelID, task.MessageType, task.AppID, task.Receiver, excludeProviderIDs)
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (h *MessageHandler) persistSelectedProvider(task *model.PushTask, providerAccountID uint) error {
	if task == nil {
		return fmt.Errorf("failed to persist selected provider account: task is nil")
	}
	if err := h.taskDao.UpdateProviderAccountID(task.TaskID, providerAccountID); err != nil {
		return fmt.Errorf("failed to persist selected provider account: %w", err)
	}
	task.ProviderAccountID = &providerAccountID
	return nil
}

// handleSuccess 处理成功
func (h *MessageHandler) handleSuccess(task *model.PushTask, providerAccountID uint, resp *sender.SendResponse, snapshot *model.SendSnapshot) error {
	// 记录日志（每次新增，便于观测请求链路），ProviderMsgID 保存在日志中用于回调匹配
	h.logDao.Create(&model.PushLog{
		TaskID:            task.TaskID,
		AppID:             task.AppID,
		ProviderAccountID: providerAccountID,
		ProviderMsgID:     resp.ProviderID,
		Status:            "success",
		RequestData:       sanitizeJSONData(resp.RequestData),
		ResponseData:      sanitizeJSONData(resp.ResponseData),
		SendSnapshot:      snapshot.JSON(),
	})

	if resp.Status == constants.TaskStatusSuccess {
		transitionResult, err := h.terminalService.Transition(context.Background(), service.TerminalTransition{
			TaskID:     task.TaskID,
			Status:     constants.TaskStatusSuccess,
			Event:      constants.WebhookEventSuccess,
			ProviderID: resp.ProviderID,
			OccurredAt: timeutil.Now(),
		})
		if err != nil {
			return fmt.Errorf("persist successful terminal task: %w", err)
		}
		if transitionResult.Changed {
			task.Status = constants.TaskStatusSuccess
		}
	} else {
		task.Status = resp.Status // sent/processing 表示仍等待供应商回执
		if err := h.taskDao.Update(task); err != nil {
			return fmt.Errorf("update non-terminal task status: %w", err)
		}
	}

	// 通知选择器成功
	h.selector.ReportSuccess(providerAccountID)

	h.logger.Info(fmt.Sprintf("message sent successfully task_id=%s provider_id=%s status=%s", task.TaskID, resp.ProviderID, resp.Status))
	return nil
}

// handleSendError 处理发送错误（使用规则引擎）
func (h *MessageHandler) handleSendError(task *model.PushTask, providerAccountID uint, resp *sender.SendResponse, snapshot *model.SendSnapshot) error {
	// 通知选择器失败
	h.selector.ReportFailure(providerAccountID)

	// 获取供应商代码
	providerCode := ""
	providerAccountDao := dao.NewProviderAccountDAO()
	if account, err := providerAccountDao.GetByID(providerAccountID); err == nil {
		providerCode = account.ProviderCode
	}

	// 使用规则引擎评估
	evalReq := &ruleengine.EvaluateRequest{
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
		SendSnapshot:      snapshot,
		Task:              task,
		ProviderAccountID: providerAccountID,
		ProviderCode:      providerCode,
		ErrorCode:         resp.ErrorCode,
		ErrorMessage:      resp.ErrorMessage,
		RequestData:       resp.RequestData,
		ResponseData:      resp.ResponseData,
		ProviderID:        resp.ProviderID,
		TerminalEvent:     constants.WebhookEventFailed,
		OccurredAt:        timeutil.Now(),
	}

	// 执行规则动作
	execResult := h.actionExecutor.Execute(context.Background(), evalResult, execCtx)

	h.logger.Info(fmt.Sprintf("rule engine executed task_id=%s action=%s retry=%v",
		task.TaskID, execResult.Action, execResult.ShouldRetry))
	return execResult.Err
}

// handleEarlyFailure 处理早期失败（发送前的错误，无供应商响应数据）
// 早期失败不使用规则引擎，直接标记失败
func (h *MessageHandler) handleEarlyFailure(task *model.PushTask, providerAccountID uint, errorMsg string, snapshot *model.SendSnapshot) {
	transitionResult, transitionErr := h.terminalService.Transition(context.Background(), service.TerminalTransition{
		TaskID:       task.TaskID,
		Status:       constants.TaskStatusFailed,
		Event:        constants.WebhookEventFailed,
		ErrorMessage: errorMsg,
		OccurredAt:   timeutil.Now(),
	})
	if transitionErr != nil {
		h.logger.Error(fmt.Sprintf("failed to persist terminal task state task_id=%s: %v", task.TaskID, transitionErr))
	} else if transitionResult.Changed {
		task.Status = constants.TaskStatusFailed
	}

	// Preparation failures are attempts too, even before a provider is selected.
	if snapshot != nil && snapshot.UnavailableReason == "尚未完成消息准备" {
		snapshot.UnavailableReason = "该次发送在消息准备前失败：" + errorMsg
	}
	if providerAccountID > 0 || snapshot != nil {
		h.logDao.Create(&model.PushLog{
			TaskID:            task.TaskID,
			AppID:             task.AppID,
			ProviderAccountID: providerAccountID,
			Status:            "failed",
			RequestData:       "{}",
			ResponseData:      "{}",
			SendSnapshot:      snapshot.JSON(),
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
