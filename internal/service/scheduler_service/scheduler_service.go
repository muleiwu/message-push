package scheduler_service

import (
	"context"

	"cnb.cool/mliev/push/message-push/app/scheduler"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// SchedulerService 调度服务
type SchedulerService struct {
	Helper            interfaces.HelperInterface
	scanner           *scheduler.ScheduledTaskScanner
	quotaSyncer       *scheduler.QuotaSyncer
	smsTimeoutScanner *scheduler.SMSTimeoutScanner
	ctx               context.Context
	cancel            context.CancelFunc
}

// Run 启动服务
func (receiver *SchedulerService) Run() error {
	// 检查系统是否已安装
	installed := receiver.Helper.GetConfig().GetBool("app.installed", false)
	if !installed {
		receiver.Helper.GetLogger().Info("系统未安装，跳过 SchedulerService 启动")
		return nil
	}

	// 创建上下文
	receiver.ctx, receiver.cancel = context.WithCancel(context.Background())

	// 创建并启动扫描器
	receiver.scanner = scheduler.NewScheduledTaskScanner()
	if err := receiver.scanner.Start(receiver.ctx); err != nil {
		return err
	}

	// 创建并启动配额同步器
	receiver.quotaSyncer = scheduler.NewQuotaSyncer()
	if err := receiver.quotaSyncer.Start(receiver.ctx); err != nil {
		return err
	}

	// 创建并启动短信超时扫描器
	receiver.smsTimeoutScanner = scheduler.NewSMSTimeoutScanner()
	if err := receiver.smsTimeoutScanner.Start(receiver.ctx); err != nil {
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

	if receiver.quotaSyncer != nil {
		receiver.quotaSyncer.Stop()
	}

	if receiver.smsTimeoutScanner != nil {
		receiver.smsTimeoutScanner.Stop()
	}

	return nil
}
