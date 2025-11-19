package scheduler

import (
	"context"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/queue"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
	"github.com/redis/go-redis/v9"
)

// ScheduledTaskScanner 定时任务扫描器
type ScheduledTaskScanner struct {
	logger   gsr.Logger
	redis    *redis.Client
	producer *queue.Producer
	taskDao  *dao.PushTaskDAO
	interval time.Duration
	stopCh   chan struct{}
}

// NewScheduledTaskScanner 创建扫描器
func NewScheduledTaskScanner() *ScheduledTaskScanner {
	h := helper.GetHelper()
	return &ScheduledTaskScanner{
		logger:   h.GetLogger(),
		redis:    h.GetRedis(),
		producer: queue.NewProducer(h.GetRedis()),
		taskDao:  dao.NewPushTaskDAO(),
		interval: 10 * time.Second, // 每10秒扫描一次
		stopCh:   make(chan struct{}),
	}
}

// Start 启动扫描器
func (s *ScheduledTaskScanner) Start(ctx context.Context) error {
	s.logger.Info("scheduled task scanner started")

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
func (s *ScheduledTaskScanner) Stop() {
	close(s.stopCh)
	s.logger.Info("scheduled task scanner stopped")
}

// scan 扫描到期任务
func (s *ScheduledTaskScanner) scan(ctx context.Context) {
	now := time.Now().Unix()
	sortedSetKey := "push:scheduled:tasks"

	// 获取到期的任务（score <= now）
	results, err := s.redis.ZRangeByScoreWithScores(ctx, sortedSetKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%d", now),
		Count: 100, // 每次最多处理100个
	}).Result()

	if err != nil {
		if err != redis.Nil {
			s.logger.Error(fmt.Sprintf("failed to scan scheduled tasks: %v", err))
		}
		return
	}

	if len(results) == 0 {
		return
	}

	s.logger.Info(fmt.Sprintf("found %d scheduled tasks to process", len(results)))

	// 处理每个到期任务
	for _, result := range results {
		taskID := result.Member.(string)

		// 从数据库获取任务
		task, err := s.taskDao.GetByTaskID(taskID)
		if err != nil {
			s.logger.Error(fmt.Sprintf("failed to get task id=%s: %v", taskID, err))
			// 删除无效的任务ID
			s.redis.ZRem(ctx, sortedSetKey, taskID)
			continue
		}

		// 推送到队列
		if err := s.producer.Push(ctx, task); err != nil {
			s.logger.Error(fmt.Sprintf("failed to push task id=%s to queue: %v", taskID, err))
			continue
		}

		// 从sorted set中删除
		s.redis.ZRem(ctx, sortedSetKey, taskID)

		s.logger.Info(fmt.Sprintf("scheduled task pushed to queue: task_id=%s", taskID))
	}
}
