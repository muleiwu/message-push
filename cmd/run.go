package cmd

import (
	"embed"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cnb.cool/mliev/examples/go-web/config"
	helper2 "cnb.cool/mliev/examples/go-web/internal/helper"
	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"cnb.cool/mliev/examples/go-web/internal/pkg/reload"
)

// Start 启动应用程序
func Start(staticFs map[string]embed.FS) {
	// 创建信号通道
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// 主循环，支持重启
	for {
		helper := helper2.GetHelper()
		servers := initializeServices(staticFs, helper)

		// 等待信号
		select {
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				// 收到 SIGHUP，执行优雅重启
				helper.GetLogger().Info("收到 SIGHUP 信号，开始重启服务...")
				stopServices(servers, helper)
				reloadConfiguration(helper)
				helper.GetLogger().Info("正在重新启动服务...")
				time.Sleep(100 * time.Millisecond) // 短暂延迟确保清理完成
				continue                           // 继续循环，重新初始化服务

			case syscall.SIGINT, syscall.SIGTERM:
				// 收到终止信号，优雅关闭并退出
				helper.GetLogger().Info(fmt.Sprintf("收到 %s 信号，开始关闭服务...", sig))
				stopServices(servers, helper)
				helper.GetLogger().Info("服务已全部关闭，程序退出")
				return
			}

		case <-reload.GetReloadChan():
			// 收到 API 触发的重启请求
			helper.GetLogger().Info("收到重启请求，开始重启服务...")
			stopServices(servers, helper)
			reloadConfiguration(helper)
			helper.GetLogger().Info("正在重新启动服务...")
			time.Sleep(100 * time.Millisecond)
			continue
		}
	}
}

// initializeServices 初始化所有服务
func initializeServices(staticFs map[string]embed.FS, helper interfaces.HelperInterface) []interfaces.ServerInterface {
	assembly := config.Assembly{
		Helper: helper,
	}
	for _, assemblyInterface := range assembly.Get() {
		err := assemblyInterface.Assembly()
		if err != nil {
			fmt.Printf("Error assembling assembly: %v\n", err)
		}
	}

	helper.GetConfig().Set("static.fs", staticFs)

	server := config.Server{
		Helper: helper,
	}
	servers := server.Get()
	for _, serverInterface := range servers {
		err := serverInterface.Run()
		if err != nil {
			helper.GetLogger().Error(err.Error())
		}
	}

	return servers
}

// stopServices 停止所有服务
func stopServices(servers []interfaces.ServerInterface, helper interfaces.HelperInterface) {
	helper.GetLogger().Info("正在停止所有服务...")
	for _, srv := range servers {
		if err := srv.Stop(); err != nil {
			helper.GetLogger().Error(fmt.Sprintf("停止服务失败: %v", err))
		}
	}
	helper.GetLogger().Info("所有服务已停止")
}

// reloadConfiguration 重新加载配置
func reloadConfiguration(helper interfaces.HelperInterface) {
	helper.GetLogger().Info("正在重新加载配置...")

	// 重新加载环境变量配置
	if env := helper.GetEnv(); env != nil {
		// 尝试调用 Reload 方法（如果实现了的话）
		type Reloader interface {
			Reload() error
		}
		if reloader, ok := env.(Reloader); ok {
			if err := reloader.Reload(); err != nil {
				helper.GetLogger().Error(fmt.Sprintf("重新加载配置失败: %v", err))
			} else {
				helper.GetLogger().Info("配置已成功重新加载")
			}
		}
	}

	// 重新初始化配置
	assembly := config.Assembly{
		Helper: helper,
	}
	for _, assemblyInterface := range assembly.Get() {
		if err := assemblyInterface.Assembly(); err != nil {
			helper.GetLogger().Error(fmt.Sprintf("重新装配服务失败: %v", err))
		}
	}
}
