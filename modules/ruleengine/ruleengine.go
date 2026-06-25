// Package ruleengine 是 ruleengine 限界上下文的对外门面（facade）。
//
// 通过类型别名复用领域类型，并提供 GetEngine() 从 go-web DI 容器解析规则引擎，
// 供 delivery / callback 等模块以接口方式调用。
package ruleengine

import (
	"cnb.cool/mliev/open/go-web/pkg/container"
	"cnb.cool/mliev/push/message-push/modules/ruleengine/domain"
)

type (
	Engine          = domain.RuleEngine
	EvaluateRequest = domain.EvaluateRequest
	EvaluateResult  = domain.EvaluateResult
)

// GetEngine 从 DI 容器解析规则引擎（domain.RuleEngine）。
// 由 modules/ruleengine/assembly 在装配阶段注册到容器。
func GetEngine() domain.RuleEngine {
	return container.MustGet[domain.RuleEngine]()
}
