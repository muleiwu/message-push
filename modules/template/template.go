// Package template 是 template 限界上下文的对外门面（facade）。
//
// 通过类型别名复用领域类型，并提供 GetRenderer()/GetService() 从 go-web DI 容器解析。
package template

import (
	"cnb.cool/mliev/open/go-web/pkg/container"
	"cnb.cool/mliev/push/message-push/modules/template/domain"
)

type (
	Renderer = domain.Renderer
	Service  = domain.Service
)

// GetRenderer 从 DI 容器解析模板渲染器（domain.Renderer）。
func GetRenderer() domain.Renderer {
	return container.MustGet[domain.Renderer]()
}

// GetService 从 DI 容器解析模板管理服务（domain.Service）。
func GetService() domain.Service {
	return container.MustGet[domain.Service]()
}
