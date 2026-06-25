// Package messaging 是 messaging 限界上下文的对外门面（facade）。
//
// 通过类型别名复用领域类型，并提供 GetService() 从 go-web DI 容器解析消息应用服务。
package messaging

import (
	"cnb.cool/mliev/open/go-web/pkg/container"
	"cnb.cool/mliev/push/message-push/modules/messaging/domain"
)

// Service 消息应用服务端口别名。
type Service = domain.Service

// GetService 从 DI 容器解析消息应用服务（domain.Service）。
// 由 modules/messaging/assembly 在装配阶段注册到容器。
func GetService() domain.Service {
	return container.MustGet[domain.Service]()
}
