package scheduler_service

import (
	"context"

	"cnb.cool/mliev/push/message-push/app/scheduler"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// SchedulerService 调度服务
type SchedulerService struct {
	Helper  interfaces.HelperInterface
	scanner *scheduler.ScheduledTaskScanner
	ctx     context.Context
	cancel  context.CancelFunc
}

// Run 启动服务
func (receiver *SchedulerService) Run() error {
	// 创建上下文
	receiver.ctx, receiver.cancel = context.WithCancel(context.Background())

	// 创建并启动扫描器
	receiver.scanner = scheduler.NewScheduledTaskScanner()
	if err := receiver.scanner.Start(receiver.ctx); err != nil {
		return err
	}

	return nil
}

// Stop 停止服务
func (receiver *SchedulerService) Stop() error {
	if receiver.cancel != nil {
		receiver.cancel()
	}

	if receiver.scanner != nil {
		receiver.scanner.Stop()
	}

	return nil
}
