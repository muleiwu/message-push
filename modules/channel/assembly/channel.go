// Package assembly 将 channel 模块的通道选择器注册到 go-web DI 容器。
package assembly

import (
	"reflect"

	"cnb.cool/mliev/push/message-push/modules/channel/domain"
	"cnb.cool/mliev/push/message-push/modules/channel/infrastructure"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
)

// Channel 实现 go-web 的 AssemblyInterface，向容器提供 domain.Selector。
type Channel struct{}

// Type 返回此 Assembly 提供的服务类型（domain.Selector 接口）。
func (receiver *Channel) Type() reflect.Type {
	return reflect.TypeFor[domain.Selector]()
}

// DependsOn 通道选择器在构造时通过 helper 访问 logger/cache/database，
// 因此须声明对这些基础设施的依赖，确保容器先于本 Assembly 完成实例化。
func (receiver *Channel) DependsOn() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[gsr.Logger](),
		reflect.TypeFor[gsr.Cacher](),
		reflect.TypeFor[*gorm.DB](),
	}
}

// Assembly 构建通道选择器（实现 domain.Selector），由框架注册到容器。
func (receiver *Channel) Assembly() (any, error) {
	return infrastructure.NewChannelSelector(), nil
}
