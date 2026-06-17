// Package domain 是 callback 限界上下文的领域层：
// 定义服务商回调处理端口（Service）。回调请求/响应复用 sender 上下文的类型。
package domain

import (
	"context"

	"cnb.cool/mliev/push/message-push/modules/sender"
)

// Service 回调处理端口：接收服务商状态回调，解析为任务状态更新、写日志、
// 触发失败规则与 Webhook 通知。
// 基础设施层提供实现，经 assembly 注册到 go-web 容器，供 callback 控制器调用。
type Service interface {
	// HandleCallback 处理服务商回调，返回服务商期望的响应实体
	HandleCallback(ctx context.Context, providerCode string, req *sender.CallbackRequest) sender.CallbackResponse
	// GetSupportedProviders 返回支持回调的服务商代码列表
	GetSupportedProviders() []string
}
