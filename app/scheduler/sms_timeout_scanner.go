package scheduler

import (
	"context"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
)

// SMSTimeoutScanner 短信超时扫描器
// 用于处理长时间处于 sent 状态未收到回调的短信任务
type SMSTimeoutScanner struct {
	logger   gsr.Logger
	taskDao  *dao.PushTaskDAO
	interval time.Duration // 扫描间隔
	timeout  time.Duration // 超时阈值
	limit    int           // 单次处理数量
	stopCh   chan struct{}
}

// NewSMSTimeoutScanner 创建短信超时扫描器
func NewSMSTimeoutScanner() *SMSTimeoutScanner {
	h := helper.GetHelper()
	return &SMSTimeoutScanner{
		logger:   h.GetLogger(),
		taskDao:  dao.NewPushTaskDAO(),
		interval: 2 * time.Second,  // 每5分钟扫描一次
		timeout:  10 * time.Second, // 24小时未收到回调视为超时
		limit:    100,              // 每次最多处理100个
		stopCh:   make(chan struct{}),
	}
}

// Start 启动扫描器
func (s *SMSTimeoutScanner) Start(ctx context.Context) error {
	s.logger.Info("sms timeout scanner started")

	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.scan(ctx)
			case <-s.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Stop 停止扫描器
func (s *SMSTimeoutScanner) Stop() {
	close(s.stopCh)
	s.logger.Info("sms timeout scanner stopped")
}

// scan 扫描超时任务
func (s *SMSTimeoutScanner) scan(ctx context.Context) {
	// 获取超时的 sent 状态任务
	tasks, err := s.taskDao.GetTimeoutSentTasks(s.timeout, s.limit)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to get timeout tasks: %v", err))
		return
	}

	if len(tasks) == 0 {
		return
	}

	s.logger.Info(fmt.Sprintf("found %d timeout sms tasks to process", len(tasks)))

	// 处理每个超时任务：保持 sent 状态，仅更新回调状态为超时
	for _, task := range tasks {
		task.CallbackStatus = constants.CallbackStatusTimeout

		now := time.Now()
		task.CallbackTime = &now

		if err := s.taskDao.Update(task); err != nil {
			s.logger.Error(fmt.Sprintf("failed to update timeout task task_id=%s: %v", task.TaskID, err))
			continue
		}

		s.logger.Info(fmt.Sprintf("timeout task callback_status marked as timeout: task_id=%s", task.TaskID))
	}
}
