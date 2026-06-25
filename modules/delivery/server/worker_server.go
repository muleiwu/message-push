// Package server 提供 go-web ServerInterface 实现：Worker 消费池与调度器。
package server

import (
	"context"
	"fmt"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/worker"
)

// WorkerServer 包装消息 Worker 池为 go-web ServerInterface。
type WorkerServer struct {
	workerPool *worker.WorkerPool
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewWorkerServer 创建 WorkerServer。
func NewWorkerServer() *WorkerServer {
	return &WorkerServer{}
}

// Run 启动 Worker 池（仅在系统已安装时）。
func (receiver *WorkerServer) Run() error {
	if !helper.GetConfig().GetBool("app.installed", false) {
		helper.GetLogger().Info("系统未安装，跳过 WorkerServer 启动")
		return nil
	}

	receiver.ctx, receiver.cancel = context.WithCancel(context.Background())

	handler := worker.NewMessageHandler()

	const poolSize = 10
	receiver.workerPool = worker.NewWorkerPool(
		poolSize,
		helper.GetRedis(),
		handler.Handle,
	)

	if err := receiver.workerPool.Start(receiver.ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	helper.GetLogger().Info(fmt.Sprintf("worker pool started with %d workers", poolSize))
	return nil
}

// Stop 停止 Worker 池。
func (receiver *WorkerServer) Stop() error {
	if receiver.cancel != nil {
		receiver.cancel()
	}
	if receiver.workerPool != nil {
		receiver.workerPool.Stop()
	}
	helper.GetLogger().Info("worker pool stopped")
	return nil
}
