package config

import (
	"cnb.cool/mliev/push/message-push/internal/interfaces"
	cacheAssembly "cnb.cool/mliev/push/message-push/internal/pkg/cache/assembly"
	configAssembly "cnb.cool/mliev/push/message-push/internal/pkg/config/assembly"
	databaseAssembly "cnb.cool/mliev/push/message-push/internal/pkg/database/assembly"
	envAssembly "cnb.cool/mliev/push/message-push/internal/pkg/env/assembly"
	loggerAssembly "cnb.cool/mliev/push/message-push/internal/pkg/logger/assembly"
	redisAssembly "cnb.cool/mliev/push/message-push/internal/pkg/redis/assembly"
)

type Assembly struct {
	Helper interfaces.HelperInterface
}

// Get 注入反转(确保注入顺序，防止依赖为空或者循环依赖)
func (receiver *Assembly) Get() []interfaces.AssemblyInterface {

	return []interfaces.AssemblyInterface{
		&envAssembly.Env{Helper: receiver.Helper}, // 环境变量
		&configAssembly.Config{ // 代码中的配置(可使用环境变量)
			Helper:         receiver.Helper, // 注入
			DefaultConfigs: Config{}.Get(),  //注入默认配置
		},
		&loggerAssembly.Logger{Helper: receiver.Helper},     // 日志驱动
		&databaseAssembly.Database{Helper: receiver.Helper}, // 数据库配置
		&redisAssembly.Redis{Helper: receiver.Helper},       // redis 配置
		&cacheAssembly.Cache{Helper: receiver.Helper},       // 缓存驱动
	}
}
