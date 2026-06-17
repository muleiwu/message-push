// Package delivery 是 delivery 限界上下文的对外门面（facade）。
//
// 通过类型别名复用领域类型，并提供 GetProducer() 从 go-web DI 容器解析消息生产者，
// 供 messaging / ruleengine 等模块以接口方式入队任务。
// 消费侧（worker 池、调度器）由 modules/delivery/server 以 go-web ServerInterface 暴露。
package delivery

import (
	"cnb.cool/mliev/open/go-web/pkg/container"
	"cnb.cool/mliev/push/message-push/modules/delivery/domain"
)

// Producer 消息生产者端口别名。
type Producer = domain.Producer

// GetProducer 从 DI 容器解析消息生产者（domain.Producer）。
// 由 modules/delivery/assembly 在装配阶段注册到容器。
func GetProducer() domain.Producer {
	return container.MustGet[domain.Producer]()
}
