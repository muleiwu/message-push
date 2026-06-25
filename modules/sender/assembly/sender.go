// Package assembly 将 sender 模块的发送器解析器注册到 go-web DI 容器。
package assembly

import (
	"reflect"

	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	"cnb.cool/mliev/push/message-push/modules/sender/infrastructure"
)

// Sender 实现 go-web 的 AssemblyInterface，向容器提供 domain.Resolver。
type Sender struct{}

// Type 返回此 Assembly 提供的服务类型（domain.Resolver 接口）。
func (receiver *Sender) Type() reflect.Type {
	return reflect.TypeFor[domain.Resolver]()
}

// DependsOn 发送器解析器构造时不依赖其他容器服务（服务商账号配置在调用时传入）。
func (receiver *Sender) DependsOn() []reflect.Type {
	return nil
}

// Assembly 构建发送器工厂（实现 domain.Resolver），由框架注册到容器。
func (receiver *Sender) Assembly() (any, error) {
	return infrastructure.NewFactory(), nil
}
