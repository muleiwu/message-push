package config

import (
	"cnb.cool/mliev/open/go-web/pkg/interfaces"
	"cnb.cool/mliev/push/message-push/server"
)

// pushAssemblies 返回 message-push 专属的 Assembly 列表。
//
// Phase 1（框架替换）阶段为空；Phase 2 将在此追加各 DDD 模块的 assembly
// （sender / channel / ruleengine / delivery 等），通过 go-web 容器按接口类型注册。
func pushAssemblies() []interfaces.AssemblyInterface {
	return []interfaces.AssemblyInterface{}
}

// pushServers 返回 message-push 专属的 Server 列表：Worker 消费池与调度器。
func pushServers() []interfaces.ServerInterface {
	return []interfaces.ServerInterface{
		server.NewWorkerServer(),
		server.NewSchedulerServer(),
	}
}
