package service

import (
	"context"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/queue"
	"cnb.cool/mliev/push/message-push/app/selector"
	internalHelper "cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/google/uuid"
	"github.com/muleiwu/gsr"
)

// MessageService 消息服务
type MessageService struct {
	logger             gsr.Logger
	producer           *queue.Producer
	selector           *selector.ChannelSelector
	taskDao            *dao.PushTaskDAO
	appDao             *dao.ApplicationDAO
	messageTemplateDao *dao.MessageTemplateDAO
	templateHelper     *helper.TemplateHelper
}

// NewMessageService 创建消息服务
func NewMessageService() *MessageService {
	h := internalHelper.GetHelper()
	return &MessageService{
		logger:             h.GetLogger(),
		producer:           queue.NewProducer(h.GetRedis()),
		selector:           selector.NewChannelSelector(),
		taskDao:            dao.NewPushTaskDAO(),
		appDao:             dao.NewApplicationDAO(),
		messageTemplateDao: dao.NewMessageTemplateDAO(),
		templateHelper:     helper.NewTemplateHelper(),
	}
}

// Send 发送消息
func (s *MessageService) Send(ctx context.Context, req *dto.SendRequest) (*dto.SendResponse, error) {
	// 1. 验证通道（获取 MessageTemplateID 和 Type）
	var channel model.Channel
	db := internalHelper.GetHelper().GetDatabase()
	if err := db.First(&channel, req.ChannelID).Error; err != nil {
		return nil, fmt.Errorf("invalid channel_id: %w", err)
	}

	// 检查通道状态
	if channel.Status != 1 {
		return nil, fmt.Errorf("channel is not active")
	}

	// 2. 验证消息类型
	if !s.isValidMessageType(channel.Type) {
		return nil, fmt.Errorf("invalid message_type: %s", channel.Type)
	}

	// 3. 加载并渲染系统模板（使用 channel 的 MessageTemplateID）
	messageTemplate, err := s.messageTemplateDao.GetByID(channel.MessageTemplateID)
	if err != nil {
		return nil, fmt.Errorf("invalid message_template_id: %w", err)
	}

	if messageTemplate.Status != 1 {
		return nil, fmt.Errorf("message template is not active")
	}

	content, err := s.templateHelper.RenderSimple(messageTemplate.Content, req.TemplateParams)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	// 4. 创建任务
	taskID := uuid.New().String()
	templateParamsJSON, _ := s.templateHelper.RenderJSON(req.TemplateParams)
	task := &model.PushTask{
		TaskID:         taskID,
		AppID:          req.AppID,
		ChannelID:      req.ChannelID,
		MessageType:    channel.Type,
		Receiver:       req.Receiver,
		Content:        content,
		TemplateCode:   "", // 将由 worker 更新为实际使用的供应商模板代码
		TemplateParams: templateParamsJSON,
		Signature:      req.SignatureName, // 用户自定义签名名称
		Status:         constants.TaskStatusPending,
		RetryCount:     0,
		MaxRetry:       3,
		ScheduledAt:    req.ScheduledAt,
		CreatedAt:      time.Now(),
	}

	// 保存任务到数据库
	if err := s.taskDao.Create(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// 5. 推送到队列
	if err := s.producer.Push(ctx, task); err != nil {
		// 更新任务状态为失败
		task.Status = constants.TaskStatusFailed
		s.taskDao.Update(task)
		return nil, fmt.Errorf("failed to push to queue: %w", err)
	}

	return &dto.SendResponse{
		TaskID:    taskID,
		Status:    constants.TaskStatusPending,
		CreatedAt: task.CreatedAt,
	}, nil
}

// BatchSend 批量发送消息
func (s *MessageService) BatchSend(ctx context.Context, req *dto.BatchSendRequest) (*dto.BatchSendResponse, error) {
	// 1. 验证通道（获取 MessageTemplateID 和 Type）
	var channel model.Channel
	db := internalHelper.GetHelper().GetDatabase()
	if err := db.First(&channel, req.ChannelID).Error; err != nil {
		return nil, fmt.Errorf("invalid channel_id: %w", err)
	}

	// 检查通道状态
	if channel.Status != 1 {
		return nil, fmt.Errorf("channel is not active")
	}

	// 2. 加载系统模板（使用 channel 的 MessageTemplateID）
	messageTemplate, err := s.messageTemplateDao.GetByID(channel.MessageTemplateID)
	if err != nil {
		return nil, fmt.Errorf("invalid message_template_id: %w", err)
	}

	if messageTemplate.Status != 1 {
		return nil, fmt.Errorf("message template is not active")
	}

	batchID := uuid.New().String()
	var tasks []*model.PushTask
	successCount := 0

	// 渲染模板内容（所有接收者共用相同的模板参数）
	content, err := s.templateHelper.RenderSimple(messageTemplate.Content, req.TemplateParams)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}
	templateParamsJSON, _ := s.templateHelper.RenderJSON(req.TemplateParams)

	for _, receiver := range req.Receivers {
		taskID := uuid.New().String()
		task := &model.PushTask{
			TaskID:         taskID,
			AppID:          req.AppID,
			ChannelID:      req.ChannelID,
			MessageType:    channel.Type,
			Receiver:       receiver,
			Content:        content,
			TemplateCode:   "", // 将由 worker 更新为实际使用的供应商模板代码
			TemplateParams: templateParamsJSON,
			Signature:      req.SignatureName, // 用户自定义签名名称
			Status:         constants.TaskStatusPending,
			RetryCount:     0,
			MaxRetry:       3,
			ScheduledAt:    req.ScheduledAt,
		}

		tasks = append(tasks, task)
	}

	// 批量保存任务
	for _, task := range tasks {
		if err := s.taskDao.Create(task); err != nil {
			s.logger.Error(fmt.Sprintf("failed to create task id=%s: %v", task.TaskID, err))
			continue
		}
		successCount++
	}

	// 批量推送到队列
	if err := s.producer.PushBatch(ctx, tasks); err != nil {
		s.logger.Error(fmt.Sprintf("failed to push batch to queue: %v", err))
	}

	return &dto.BatchSendResponse{
		BatchID:      batchID,
		TotalCount:   len(req.Receivers),
		SuccessCount: successCount,
		FailedCount:  len(req.Receivers) - successCount,
		CreatedAt:    time.Now(),
	}, nil
}

// QueryTask 查询任务状态
func (s *MessageService) QueryTask(ctx context.Context, taskID string) (*model.PushTask, error) {
	task, err := s.taskDao.GetByTaskID(taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	return task, nil
}

// isValidMessageType 验证消息类型
func (s *MessageService) isValidMessageType(messageType string) bool {
	validTypes := []string{
		constants.MessageTypeSMS,
		constants.MessageTypeEmail,
		constants.MessageTypeWeChatWork,
		constants.MessageTypeDingTalk,
	}

	for _, t := range validTypes {
		if t == messageType {
			return true
		}
	}
	return false
}
