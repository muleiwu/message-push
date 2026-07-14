package config

import (
	"cnb.cool/mliev/open/go-web/pkg/interfaces"
	cacheAssembly "cnb.cool/mliev/open/go-web/pkg/server/cache/assembly"
	configAssembly "cnb.cool/mliev/open/go-web/pkg/server/config/assembly"
	databaseAssembly "cnb.cool/mliev/open/go-web/pkg/server/database/assembly"
	envAssembly "cnb.cool/mliev/open/go-web/pkg/server/env/assembly"
	httpService "cnb.cool/mliev/open/go-web/pkg/server/http_server/service"
	loggerAssembly "cnb.cool/mliev/open/go-web/pkg/server/logger/assembly"
	redisAssembly "cnb.cool/mliev/open/go-web/pkg/server/redis/assembly"
	"cnb.cool/mliev/push/message-push/migration"
	identityServer "cnb.cool/mliev/push/message-push/modules/identity/server"
)

// App 是 message-push 的 AppProvider 实现，声明 Assembly 与 Server 链。
type App struct{}

// Assemblies 返回 DI 装配链。
//
// 基础设施（env/config/logger/database/redis/cache）由 go-web 提供；
// message-push 专属扩展在 app_push.go 的 pushAssemblies() 中追加（DDD 模块装配在 Phase 2 接入）。
func (a App) Assemblies() []interfaces.AssemblyInterface {
	base := []interfaces.AssemblyInterface{
		&envAssembly.Env{},
		&configAssembly.Config{DefaultConfigs: Config{}.Get()},
		&loggerAssembly.Logger{},
		&databaseAssembly.Database{},
		&redisAssembly.Redis{},
		&cacheAssembly.Cache{},
	}
	base = append(base, pushAssemblies()...)
	return base
}

// Servers 返回 Server 链（迁移 → 初始管理员引导 → worker/调度器 → HTTP 服务）。
func (a App) Servers() []interfaces.ServerInterface {
	servers := []interfaces.ServerInterface{
		identityServer.NewJWTConfigServer(),
		&migration.Server{},
		identityServer.NewBootstrapAdminServer(),
	}
	servers = append(servers, pushServers()...)
	servers = append(servers, &httpService.HttpServer{})
	return servers
}
