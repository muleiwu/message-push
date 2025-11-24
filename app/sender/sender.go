package sender

import (
	"context"

	"cnb.cool/mliev/push/message-push/app/model"
)

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
}

// Sender 发送器接口
type Sender interface {
	// Send 发送消息
	Send(ctx context.Context, req *SendRequest) (*SendResponse, error)

	// GetType 获取发送器类型
	GetType() string
}
