package service

import (
	"context"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
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
	signatureHelper    *helper.SignatureHelper
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
		signatureHelper:    helper.NewSignatureHelper(),
	}
}

// SendRequest 发送请求参数
type SendRequest struct {
	AppID             string                 `json:"app_id" binding:"required"`
	ChannelID         uint                   `json:"channel_id" binding:"required"`
	MessageType       string                 `json:"message_type" binding:"required"`
	Receiver          string                 `json:"receiver" binding:"required"`
	MessageTemplateID uint                   `json:"message_template_id" binding:"required"`
	TemplateParams    map[string]interface{} `json:"template_params"`
	ScheduledAt       *time.Time             `json:"scheduled_at"`
	Signature         string                 `json:"signature" binding:"required"`
	Timestamp         int64                  `json:"timestamp" binding:"required"`
}

// SendResponse 发送响应
type SendResponse struct {
	TaskID    string    `json:"task_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Send 发送消息
func (s *MessageService) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	// 1. 验证应用
	app, err := s.appDao.GetByAppID(req.AppID)
	if err != nil {
		return nil, fmt.Errorf("invalid app_id: %w", err)
	}

	if app.Status != 1 {
		return nil, fmt.Errorf("app is not active")
	}

	// 2. 验证签名
	appSecret, err := helper.DecryptAppSecret(app.AppSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt app secret: %w", err)
	}

	body := []byte(fmt.Sprintf("%d|%s|%d|%s", req.ChannelID, req.Receiver, req.MessageTemplateID, req.TemplateParams))
	if !s.signatureHelper.VerifySignature(req.AppID, appSecret, req.Signature, req.Timestamp, body) {
		return nil, fmt.Errorf("invalid signature")
	}

	// 3. 验证通道
	var channel model.Channel
	db := internalHelper.GetHelper().GetDatabase()
	if err := db.First(&channel, req.ChannelID).Error; err != nil {
		return nil, fmt.Errorf("invalid channel_id: %w", err)
	}

	// 检查通道状态
	if channel.Status != 1 {
		return nil, fmt.Errorf("channel is not active")
	}

	// 4. 验证消息类型
	if !s.isValidMessageType(req.MessageType) {
		return nil, fmt.Errorf("invalid message_type: %s", req.MessageType)
	}

	// 5. 加载并渲染系统模板
	messageTemplate, err := s.messageTemplateDao.GetByID(req.MessageTemplateID)
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

	// 6. 创建任务
	taskID := uuid.New().String()
	templateParamsJSON, _ := s.templateHelper.RenderJSON(req.TemplateParams)
	task := &model.PushTask{
		TaskID:         taskID,
		AppID:          app.AppID,
		ChannelID:      req.ChannelID,
		MessageType:    req.MessageType,
		Receiver:       req.Receiver,
		Content:        content,
		TemplateCode:   "", // 将由 worker 更新为实际使用的供应商模板代码
		TemplateParams: templateParamsJSON,
		Status:         constants.TaskStatusPending,
		RetryCount:     0,
		MaxRetry:       3,
		ScheduledAt:    req.ScheduledAt,
		Signature:      req.Signature,
	}

	// 保存任务到数据库
	if err := s.taskDao.Create(task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// 7. 推送到队列
	if err := s.producer.Push(ctx, task); err != nil {
		// 更新任务状态为失败
		task.Status = constants.TaskStatusFailed
		s.taskDao.Update(task)
		return nil, fmt.Errorf("failed to push to queue: %w", err)
	}

	return &SendResponse{
		TaskID:    taskID,
		Status:    constants.TaskStatusPending,
		CreatedAt: task.CreatedAt,
	}, nil
}

// BatchSendRequest 批量发送请求
type BatchSendRequest struct {
	AppID             string                   `json:"app_id" binding:"required"`
	ChannelID         uint                     `json:"channel_id" binding:"required"`
	MessageType       string                   `json:"message_type" binding:"required"`
	MessageTemplateID uint                     `json:"message_template_id" binding:"required"`
	Receivers         []map[string]interface{} `json:"receivers" binding:"required"` // [{receiver, params}, ...]
	Signature         string                   `json:"signature" binding:"required"`
	Timestamp         int64                    `json:"timestamp" binding:"required"`
}

// BatchSendResponse 批量发送响应
type BatchSendResponse struct {
	BatchID      string    `json:"batch_id"`
	TotalCount   int       `json:"total_count"`
	SuccessCount int       `json:"success_count"`
	FailedCount  int       `json:"failed_count"`
	CreatedAt    time.Time `json:"created_at"`
}

// BatchSend 批量发送消息
func (s *MessageService) BatchSend(ctx context.Context, req *BatchSendRequest) (*BatchSendResponse, error) {
	// 验证应用
	app, err := s.appDao.GetByAppID(req.AppID)
	if err != nil {
		return nil, fmt.Errorf("invalid app_id: %w", err)
	}

	// 加载系统模板
	messageTemplate, err := s.messageTemplateDao.GetByID(req.MessageTemplateID)
	if err != nil {
		return nil, fmt.Errorf("invalid message_template_id: %w", err)
	}

	if messageTemplate.Status != 1 {
		return nil, fmt.Errorf("message template is not active")
	}

	batchID := uuid.New().String()
	var tasks []*model.PushTask
	successCount := 0

	for _, item := range req.Receivers {
		receiver, _ := item["receiver"].(string)
		params, _ := item["params"].(map[string]interface{})

		content, err := s.templateHelper.RenderSimple(messageTemplate.Content, params)
		if err != nil {
			s.logger.Error(fmt.Sprintf("failed to render template for receiver=%s: %v", receiver, err))
			continue
		}

		taskID := uuid.New().String()
		templateParamsJSON, _ := s.templateHelper.RenderJSON(params)
		task := &model.PushTask{
			TaskID:         taskID,
			AppID:          app.AppID,
			ChannelID:      req.ChannelID,
			MessageType:    req.MessageType,
			Receiver:       receiver,
			Content:        content,
			TemplateCode:   "", // 将由 worker 更新为实际使用的供应商模板代码
			TemplateParams: templateParamsJSON,
			Status:         constants.TaskStatusPending,
			RetryCount:     0,
			MaxRetry:       3,
			Signature:      req.Signature,
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

	return &BatchSendResponse{
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
