package dto

import "time"

// SendRequest 发送请求参数
type SendRequest struct {
	AppID          string                 `json:"app_id"`
	ChannelID      uint                   `json:"channel_id" binding:"required"`
	Receiver       string                 `json:"receiver" binding:"required"`
	TemplateParams map[string]interface{} `json:"template_params"`
	SignatureName  string                 `json:"signature_name"` // 用户自定义签名名称
	ScheduledAt    *time.Time             `json:"scheduled_at"`
}

// BatchSendRequest 批量发送请求
type BatchSendRequest struct {
	AppID          string                 `json:"app_id"`
	ChannelID      uint                   `json:"channel_id" binding:"required"`
	Receivers      []string               `json:"receivers" binding:"required"` // 手机号数组
	TemplateParams map[string]interface{} `json:"template_params"`              // 模板参数（所有接收者共用）
	SignatureName  string                 `json:"signature_name"`               // 用户自定义签名名称
	ScheduledAt    *time.Time             `json:"scheduled_at"`
}

// SendResponse 发送响应
type SendResponse struct {
	TaskID    string    `json:"task_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// BatchSendResponse 批量发送响应
type BatchSendResponse struct {
	BatchID      string    `json:"batch_id"`
	TotalCount   int       `json:"total_count"`
	SuccessCount int       `json:"success_count"`
	FailedCount  int       `json:"failed_count"`
	CreatedAt    time.Time `json:"created_at"`
}
