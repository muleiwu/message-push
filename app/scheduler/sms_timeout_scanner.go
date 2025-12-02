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

// SMSTimeoutScanner 消息超时扫描器
// 用于处理：
// 1. 长时间处于 sent 状态未收到回调的短信任务
// 2. 长时间处于 processing 状态的任务（如邮件发送阻塞）
type SMSTimeoutScanner struct {
	logger            gsr.Logger
	taskDao           *dao.PushTaskDAO
	interval          time.Duration // 扫描间隔
	timeout           time.Duration // sent 状态超时阈值（短信回调）
	processingTimeout time.Duration // processing 状态超时阈值（发送阻塞）
	limit             int           // 单次处理数量
	stopCh            chan struct{}
}

// NewSMSTimeoutScanner 创建消息超时扫描器
func NewSMSTimeoutScanner() *SMSTimeoutScanner {
	h := helper.GetHelper()
	return &SMSTimeoutScanner{
		logger:            h.GetLogger(),
		taskDao:           dao.NewPushTaskDAO(),
		interval:          10 * time.Second,  // 每10秒扫描一次
		timeout:           60 * time.Second,  // 60秒未收到回调视为超时
		processingTimeout: 120 * time.Second, // 120秒处理中视为超时（考虑发送重试等情况）
		limit:             100,               // 每次最多处理100个
		stopCh:            make(chan struct{}),
	}
}

// Start 启动扫描器
func (s *SMSTimeoutScanner) Start(ctx context.Context) error {
	s.logger.Info("message timeout scanner started")

	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.scanSentTasks(ctx)
				s.scanProcessingTasks(ctx)
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
	s.logger.Info("message timeout scanner stopped")
}

// scanSentTasks 扫描超时的 sent 状态短信任务
func (s *SMSTimeoutScanner) scanSentTasks(ctx context.Context) {
	// 获取超时的 sent 状态任务
	tasks, err := s.taskDao.GetTimeoutSentTasks(s.timeout, s.limit)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to get timeout sent tasks: %v", err))
		return
	}

	if len(tasks) == 0 {
		return
	}

	s.logger.Info(fmt.Sprintf("found %d timeout sent tasks to process", len(tasks)))

	// 处理每个超时任务：保持 sent 状态，仅更新回调状态为超时
	for _, task := range tasks {
		task.CallbackStatus = constants.CallbackStatusTimeout

		now := time.Now()
		task.CallbackTime = &now

		if err := s.taskDao.Update(task); err != nil {
			s.logger.Error(fmt.Sprintf("failed to update timeout sent task task_id=%s: %v", task.TaskID, err))
			continue
		}

		s.logger.Info(fmt.Sprintf("sent task callback_status marked as timeout: task_id=%s", task.TaskID))
	}
}

// scanProcessingTasks 扫描超时的 processing 状态任务（所有消息类型）
func (s *SMSTimeoutScanner) scanProcessingTasks(ctx context.Context) {
	// 获取超时的 processing 状态任务
	tasks, err := s.taskDao.GetTimeoutProcessingTasks(s.processingTimeout, s.limit)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to get timeout processing tasks: %v", err))
		return
	}

	if len(tasks) == 0 {
		return
	}

	s.logger.Info(fmt.Sprintf("found %d timeout processing tasks to process", len(tasks)))

	// 处理每个超时任务：标记为失败
	for _, task := range tasks {
		task.Status = constants.TaskStatusFailed

		if err := s.taskDao.Update(task); err != nil {
			s.logger.Error(fmt.Sprintf("failed to update timeout processing task task_id=%s: %v", task.TaskID, err))
			continue
		}

		s.logger.Info(fmt.Sprintf("processing task marked as failed due to timeout: task_id=%s message_type=%s", task.TaskID, task.MessageType))
	}
}
