package scheduler

import (
	"context"
	"fmt"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/lock"
	"github.com/muleiwu/gsr"
)

// SMSTimeoutScanner 消息超时扫描器
// 用于处理：
// 1. 长时间处于 sent 状态未收到回调的短信任务
// 2. 长时间处于 processing 状态的任务（如邮件发送阻塞）
// 分布式安全：tick 级分布式锁保证同一时刻只有一个实例扫描；
// DAO 层使用条件化 UPDATE（CAS），即使锁失效并发执行也不会覆盖回调结果。
type SMSTimeoutScanner struct {
	logger            gsr.Logger
	taskDao           *dao.PushTaskDAO
	lock              *lock.RedisLock
	interval          time.Duration // 扫描间隔
	timeout           time.Duration // sent 状态超时阈值（短信回调）
	processingTimeout time.Duration // processing 状态超时阈值（发送阻塞）
	limit             int           // 单次处理数量
	stopCh            chan struct{}
}

// NewSMSTimeoutScanner 创建消息超时扫描器
func NewSMSTimeoutScanner() *SMSTimeoutScanner {
	return &SMSTimeoutScanner{
		logger:            helper.GetLogger(),
		taskDao:           dao.NewPushTaskDAO(),
		lock:              lock.NewRedisLock(helper.GetRedis(), "sms_timeout_scanner", 30*time.Second), // TTL 须大于两条有界 UPDATE 的耗时
		interval:          10 * time.Second,                                                            // 每10秒扫描一次
		timeout:           60 * time.Second,                                                            // 60秒未收到回调视为超时
		processingTimeout: 120 * time.Second,                                                           // 120秒处理中视为超时（考虑发送重试等情况）
		limit:             100,                                                                         // 每次最多处理100个
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
				// 分布式锁：拿不到说明其他实例正在扫描（或 Redis 异常），跳过本轮
				ok, err := s.lock.TryLock(ctx)
				if err != nil || !ok {
					continue
				}
				s.scanSentTasks(ctx)
				s.scanProcessingTasks(ctx)
				_ = s.lock.Unlock(ctx)
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

// scanSentTasks 扫描超时的 sent 状态短信任务：保持 sent 状态，仅将回调状态置为超时
func (s *SMSTimeoutScanner) scanSentTasks(ctx context.Context) {
	affected, err := s.taskDao.MarkTimeoutSentTasksCallback(s.timeout, s.limit)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to mark timeout sent tasks: %v", err))
		return
	}

	if affected > 0 {
		s.logger.Info(fmt.Sprintf("marked %d sent tasks callback_status as timeout", affected))
	}
}

// scanProcessingTasks 扫描超时的 processing 状态任务（所有消息类型）：标记为失败
func (s *SMSTimeoutScanner) scanProcessingTasks(ctx context.Context) {
	affected, err := s.taskDao.MarkTimeoutProcessingTasksFailed(s.processingTimeout, s.limit)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to mark timeout processing tasks: %v", err))
		return
	}

	if affected > 0 {
		s.logger.Info(fmt.Sprintf("marked %d processing tasks as failed due to timeout", affected))
	}
}
