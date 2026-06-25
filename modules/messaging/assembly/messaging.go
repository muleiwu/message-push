// Package assembly 将 messaging 模块的消息应用服务注册到 go-web DI 容器。
package assembly

import (
	"reflect"

	channelDomain "cnb.cool/mliev/push/message-push/modules/channel/domain"
	deliveryDomain "cnb.cool/mliev/push/message-push/modules/delivery/domain"
	"cnb.cool/mliev/push/message-push/modules/messaging/domain"
	"cnb.cool/mliev/push/message-push/modules/messaging/infrastructure"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
)

// Messaging 实现 go-web 的 AssemblyInterface，向容器提供 domain.Service。
type Messaging struct{}

// Type 返回此 Assembly 提供的服务类型（domain.Service 接口）。
func (receiver *Messaging) Type() reflect.Type {
	return reflect.TypeFor[domain.Service]()
}

// DependsOn 消息服务构造时经 helper 访问 logger/database，并从容器解析
// delivery.Producer 与 channel.Selector，故须声明这些依赖以保证装配顺序。
func (receiver *Messaging) DependsOn() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[gsr.Logger](),
		reflect.TypeFor[*gorm.DB](),
		reflect.TypeFor[deliveryDomain.Producer](),
		reflect.TypeFor[channelDomain.Selector](),
	}
}

// Assembly 构建消息应用服务（实现 domain.Service），由框架注册到容器。
func (receiver *Messaging) Assembly() (any, error) {
	return infrastructure.NewMessageService(), nil
}
