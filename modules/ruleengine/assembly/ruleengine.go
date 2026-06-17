// Package assembly 将 ruleengine 模块的规则引擎注册到 go-web DI 容器。
package assembly

import (
	"reflect"

	"cnb.cool/mliev/push/message-push/modules/ruleengine/domain"
	"cnb.cool/mliev/push/message-push/modules/ruleengine/infrastructure"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
)

// RuleEngine 实现 go-web 的 AssemblyInterface，向容器提供 domain.RuleEngine。
type RuleEngine struct{}

// Type 返回此 Assembly 提供的服务类型（domain.RuleEngine 接口）。
func (receiver *RuleEngine) Type() reflect.Type {
	return reflect.TypeFor[domain.RuleEngine]()
}

// DependsOn 规则引擎在构造时通过 helper 访问 logger，并加载 DB 规则到缓存，
// 因此须声明对 logger 与 database 的依赖。
func (receiver *RuleEngine) DependsOn() []reflect.Type {
	return []reflect.Type{
		reflect.TypeFor[gsr.Logger](),
		reflect.TypeFor[*gorm.DB](),
	}
}

// Assembly 构建规则引擎（实现 domain.RuleEngine），由框架注册到容器。
func (receiver *RuleEngine) Assembly() (any, error) {
	return infrastructure.New(), nil
}
