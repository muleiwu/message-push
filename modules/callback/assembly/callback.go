// Package assembly 将 callback 模块的回调处理服务注册到 go-web DI 容器。
package assembly

import (
	"reflect"

	"cnb.cool/mliev/push/message-push/modules/callback/domain"
	"cnb.cool/mliev/push/message-push/modules/callback/infrastructure"
	deliveryDomain "cnb.cool/mliev/push/message-push/modules/delivery/domain"
	ruleengineDomain "cnb.cool/mliev/push/message-push/modules/ruleengine/domain"
	senderDomain "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
)

// Callback 实现 go-web 的 AssemblyInterface，向容器提供 domain.Service。
type Callback struct{}

// Type 返回此 Assembly 提供的服务类型（domain.Service 接口）。
func (receiver *Callback) Type() reflect.Type {
	return reflect.TypeFor[domain.Service]()
}

// DependsOn 回调服务构造时经 helper 访问 logger/database，并从容器解析
// sender.Resolver、ruleengine.RuleEngine，以及（经 ActionExecutor）delivery.Producer。
func (receiver *Callback) DependsOn() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[gsr.Logger](),
		reflect.TypeFor[*gorm.DB](),
		reflect.TypeFor[senderDomain.Resolver](),
		reflect.TypeFor[ruleengineDomain.RuleEngine](),
		reflect.TypeFor[deliveryDomain.Producer](),
	}
}

// Assembly 构建回调处理服务（实现 domain.Service），由框架注册到容器。
func (receiver *Callback) Assembly() (any, error) {
	return infrastructure.NewCallbackService(), nil
}
