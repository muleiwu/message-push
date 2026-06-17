// Package callback 是 callback 限界上下文的对外门面（facade）。
package callback

import (
	"cnb.cool/mliev/open/go-web/pkg/container"
	"cnb.cool/mliev/push/message-push/modules/callback/domain"
)

// Service 回调处理服务端口别名。
type Service = domain.Service

// GetService 从 DI 容器解析回调处理服务（domain.Service）。
func GetService() domain.Service {
	return container.MustGet[domain.Service]()
}
