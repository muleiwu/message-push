package sender

import (
	"context"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
)

// ==================== 发送相关 ====================

// SendRequest 发送请求
type SendRequest struct {
	Task            *model.PushTask
	ProviderChannel *model.ProviderChannel
	Provider        *model.Provider
	Relation        *model.ChannelProviderRelation // 关联配置（签名/模板等）
	Signature       *model.ProviderSignature       // 签名配置（用于SMS类型）
}

// SendResponse 发送响应
type SendResponse struct {
	Success      bool
	ProviderID   string // 服务商返回的消息ID
	ErrorCode    string
	ErrorMessage string
	TaskID       string // 关联的任务ID（批量发送时使用）
}

// Sender 发送器接口
type Sender interface {
	// Send 发送消息
	Send(ctx context.Context, req *SendRequest) (*SendResponse, error)

	// GetType 获取发送器类型
	GetType() string
}

// ==================== 批量发送相关 ====================

// BatchSendRequest 批量发送请求
type BatchSendRequest struct {
	Tasks           []*model.PushTask
	ProviderChannel *model.ProviderChannel
	Provider        *model.Provider
	Relation        *model.ChannelProviderRelation
	Signature       *model.ProviderSignature
}

// BatchSendResponse 批量发送响应
type BatchSendResponse struct {
	Results []*SendResponse // 每个任务的发送结果
}

// BatchSender 批量发送器接口（可选实现）
type BatchSender interface {
	Sender
	// BatchSend 批量发送消息
	BatchSend(ctx context.Context, req *BatchSendRequest) (*BatchSendResponse, error)
	// SupportsBatchSend 是否支持批量发送
	SupportsBatchSend() bool
}

// ==================== 回调处理相关 ====================

// CallbackRequest 回调请求
type CallbackRequest struct {
	ProviderCode string            // 服务商代码
	RawBody      []byte            // 原始请求体
	Headers      map[string]string // 请求头（用于签名验证等）
	QueryParams  map[string]string // URL 查询参数
}

// CallbackResult 回调结果
type CallbackResult struct {
	TaskID       string    // 关联的任务ID（通过 ProviderID 反查）
	ProviderID   string    // 服务商消息ID
	Status       string    // 状态：delivered, failed, rejected
	ErrorCode    string    // 错误码
	ErrorMessage string    // 错误信息
	ReportTime   time.Time // 回调时间
}

// CallbackHandler 回调处理器接口
type CallbackHandler interface {
	// HandleCallback 处理服务商回调
	HandleCallback(ctx context.Context, req *CallbackRequest) ([]*CallbackResult, error)
	// GetProviderCode 获取服务商代码
	GetProviderCode() string
	// SupportsCallback 是否支持回调
	SupportsCallback() bool
}
