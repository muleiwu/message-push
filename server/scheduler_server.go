package server

import (
	"context"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/scheduler"
)

// SchedulerServer 包装调度器（定时任务扫描、配额同步、短信超时扫描）为 go-web ServerInterface。
type SchedulerServer struct {
	scanner           *scheduler.ScheduledTaskScanner
	quotaSyncer       *scheduler.QuotaSyncer
	smsTimeoutScanner *scheduler.SMSTimeoutScanner
	ctx               context.Context
	cancel            context.CancelFunc
}

// NewSchedulerServer 创建 SchedulerServer。
func NewSchedulerServer() *SchedulerServer {
	return &SchedulerServer{}
}

// Run 启动所有调度器（仅在系统已安装时）。
func (receiver *SchedulerServer) Run() error {
	if !helper.GetConfig().GetBool("app.installed", false) {
		helper.GetLogger().Info("系统未安装，跳过 SchedulerServer 启动")
		return nil
	}

	receiver.ctx, receiver.cancel = context.WithCancel(context.Background())

	receiver.scanner = scheduler.NewScheduledTaskScanner()
	if err := receiver.scanner.Start(receiver.ctx); err != nil {
		return err
	}

	receiver.quotaSyncer = scheduler.NewQuotaSyncer()
	if err := receiver.quotaSyncer.Start(receiver.ctx); err != nil {
		return err
	}

	receiver.smsTimeoutScanner = scheduler.NewSMSTimeoutScanner()
	if err := receiver.smsTimeoutScanner.Start(receiver.ctx); err != nil {
		return err
	}

	return nil
}

// Stop 停止所有调度器。
func (receiver *SchedulerServer) Stop() error {
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
