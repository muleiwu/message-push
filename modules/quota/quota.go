// Package quota 是 quota 限界上下文的对外门面（facade）。
package quota

import (
	"cnb.cool/mliev/open/go-web/pkg/container"
	"cnb.cool/mliev/push/message-push/modules/quota/domain"
)

// Service 配额服务端口别名。
type Service = domain.Service

// GetService 从 DI 容器解析配额服务（domain.Service）。
func GetService() domain.Service {
	return container.MustGet[domain.Service]()
}
