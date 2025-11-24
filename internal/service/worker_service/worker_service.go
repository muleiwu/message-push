package worker_service

import (
	"context"
	"fmt"

	"cnb.cool/mliev/push/message-push/app/worker"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// WorkerService Worker服务
type WorkerService struct {
	Helper     interfaces.HelperInterface
	workerPool *worker.WorkerPool
	ctx        context.Context
	cancel     context.CancelFunc
}

// Run 启动服务
func (receiver *WorkerService) Run() error {
	// 检查系统是否已安装
	installed := receiver.Helper.GetConfig().GetBool("app.installed", false)
	if !installed {
		receiver.Helper.GetLogger().Info("系统未安装，跳过 WorkerService 启动")
		return nil
	}

	// 创建上下文
	receiver.ctx, receiver.cancel = context.WithCancel(context.Background())

	// 创建消息处理器
	handler := worker.NewMessageHandler()

	// 创建WorkerPool (默认10个worker)
	poolSize := 10
	receiver.workerPool = worker.NewWorkerPool(
		poolSize,
		receiver.Helper.GetRedis(),
		handler.Handle,
	)

	// 启动WorkerPool
	if err := receiver.workerPool.Start(receiver.ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	receiver.Helper.GetLogger().Info(fmt.Sprintf("worker pool started with %d workers", poolSize))

	return nil
}

// Stop 停止服务
func (receiver *WorkerService) Stop() error {
	if receiver.cancel != nil {
		receiver.cancel()
	}

	if receiver.workerPool != nil {
		receiver.workerPool.Stop()
	}

	receiver.Helper.GetLogger().Info("worker pool stopped")
	return nil
}
