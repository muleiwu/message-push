package config

import (
	"cnb.cool/mliev/open/go-web/pkg/interfaces"
	callbackAssembly "cnb.cool/mliev/push/message-push/modules/callback/assembly"
	channelAssembly "cnb.cool/mliev/push/message-push/modules/channel/assembly"
	deliveryAssembly "cnb.cool/mliev/push/message-push/modules/delivery/assembly"
	deliveryServer "cnb.cool/mliev/push/message-push/modules/delivery/server"
	identityAssembly "cnb.cool/mliev/push/message-push/modules/identity/assembly"
	messagingAssembly "cnb.cool/mliev/push/message-push/modules/messaging/assembly"
	quotaAssembly "cnb.cool/mliev/push/message-push/modules/quota/assembly"
	ruleengineAssembly "cnb.cool/mliev/push/message-push/modules/ruleengine/assembly"
	senderAssembly "cnb.cool/mliev/push/message-push/modules/sender/assembly"
	templateAssembly "cnb.cool/mliev/push/message-push/modules/template/assembly"
)

// pushAssemblies 返回 message-push 专属的 Assembly 列表。
//
// Phase 2 起在此追加各 DDD 模块的 assembly（sender / channel / ruleengine / delivery 等），
// 通过 go-web 容器按接口类型注册，供其他模块以接口方式解析调用。
func pushAssemblies() []interfaces.AssemblyInterface {
	return []interfaces.AssemblyInterface{
		&quotaAssembly.Quota{},
		&senderAssembly.Sender{},
		&channelAssembly.Channel{},
		&ruleengineAssembly.RuleEngine{},
		&deliveryAssembly.Producer{},
		&templateAssembly.Renderer{},
		&templateAssembly.Service{},
		&messagingAssembly.Messaging{},
		&callbackAssembly.Callback{},
		&identityAssembly.ApplicationService{},
		&identityAssembly.UserService{},
		&identityAssembly.OIDCService{},
	}
}

// pushServers 返回 message-push 专属的 Server 列表：Worker 消费池与调度器（delivery 模块）。
func pushServers() []interfaces.ServerInterface {
	return []interfaces.ServerInterface{
		deliveryServer.NewWorkerServer(),
		deliveryServer.NewSchedulerServer(),
	}
}
