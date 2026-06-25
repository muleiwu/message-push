// Package channel 是 channel 限界上下文的对外门面（facade）。
//
// 通过类型别名复用领域类型，并提供 GetSelector() 从 go-web DI 容器解析通道选择器，
// 供 messaging / delivery 等模块以接口方式调用，实现模块解耦。
package channel

import (
	"cnb.cool/mliev/open/go-web/pkg/container"
	"cnb.cool/mliev/push/message-push/modules/channel/domain"
)

type (
	Selector    = domain.Selector
	ChannelNode = domain.ChannelNode
)

// GetSelector 从 DI 容器解析通道选择器（domain.Selector）。
// 由 modules/channel/assembly 在装配阶段注册到容器。
func GetSelector() domain.Selector {
	return container.MustGet[domain.Selector]()
}
