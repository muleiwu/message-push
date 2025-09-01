package cmd

import (
	"embed"
	"os"
	"os/signal"
	"syscall"

	"cnb.cool/mliev/examples/go-web/config"
	helper2 "cnb.cool/mliev/examples/go-web/internal/helper"
)

// Start 启动应用程序
func Start(staticFs map[string]embed.FS) {
	initializeServices(staticFs)
	// 添加阻塞以保持主程序运行
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
}

// initializeServices 初始化所有服务
func initializeServices(staticFs map[string]embed.FS) {

	helper := helper2.NewHelper()

	assembly := config.Assembly{
		Helper: helper,
	}
	for _, assemblyInterface := range assembly.Get() {
		assemblyInterface.Assembly()
	}

	helper.GetConfig().Set("file.static", staticFs)

	server := config.Server{
		Helper: helper,
	}
	for _, serverInterface := range server.Get() {
		err := serverInterface.Run()
		if err != nil {
			helper.GetLogger().Error(err.Error())
		}
	}
}
