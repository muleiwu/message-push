package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/queue"
	"github.com/muleiwu/gsr"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// claimScript 租约认领脚本：仅当成员仍到期（score <= now）时，将其 score 原子推到租约时间。
// 并发下只有一个实例能返回 1，实现多实例互斥抢占；
// 若持有者在推送前崩溃，租约到期后任务重新可见，不丢消息（at-least-once）。
var claimScript = redis.NewScript(`
local score = redis.call('ZSCORE', KEYS[1], ARGV[1])
if score and tonumber(score) <= tonumber(ARGV[2]) then
    redis.call('ZADD', KEYS[1], ARGV[3], ARGV[1])
    return 1
end
return 0
`)

// ScheduledTaskScanner 定时任务扫描器
type ScheduledTaskScanner struct {
	logger   gsr.Logger
	redis    *redis.Client
	producer *queue.Producer
	taskDao  *dao.PushTaskDAO
	interval time.Duration
	lease    time.Duration // 认领租约时长，须大于「DB 查询 + 推送队列」的最长耗时
	stopCh   chan struct{}
}

// NewScheduledTaskScanner 创建扫描器
func NewScheduledTaskScanner() *ScheduledTaskScanner {
	return &ScheduledTaskScanner{
		logger:   helper.GetLogger(),
		redis:    helper.GetRedis(),
		producer: queue.NewProducer(helper.GetRedis()),
		taskDao:  dao.NewPushTaskDAO(),
		interval: 10 * time.Second, // 每10秒扫描一次
		lease:    30 * time.Second, // 认领租约30秒
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
		origScore := result.Score

		// 原子认领：把 score 推到 now+lease，返回 0 说明已被其他实例认领，跳过
		claimed, err := claimScript.Run(ctx, s.redis, []string{sortedSetKey},
			taskID, now, now+int64(s.lease.Seconds())).Int()
		if err != nil {
			s.logger.Error(fmt.Sprintf("failed to claim scheduled task id=%s: %v", taskID, err))
			continue
		}
		if claimed != 1 {
			continue
		}

		// 从数据库获取任务
		task, err := s.taskDao.GetByTaskID(taskID)
		if err != nil {
			s.logger.Error(fmt.Sprintf("failed to get task id=%s: %v", taskID, err))
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 任务确实不存在，删除无效的任务ID
				s.redis.ZRem(ctx, sortedSetKey, taskID)
			} else {
				// 瞬时数据库错误：回补原 score 等下轮重试，不能误删排期
				s.redis.ZAdd(ctx, sortedSetKey, redis.Z{Score: origScore, Member: taskID})
			}
			continue
		}

		// 推送到队列；失败则回补原 score 等待下轮重试（进程崩溃场景由租约到期兜底）
		if err := s.producer.Push(ctx, task); err != nil {
			s.logger.Error(fmt.Sprintf("failed to push task id=%s to queue: %v", taskID, err))
			s.redis.ZAdd(ctx, sortedSetKey, redis.Z{Score: origScore, Member: taskID})
			continue
		}

		// 从sorted set中删除；若任务已被改期到未来，Push 内部会写入新 score，此时不能误删
		if task.ScheduledAt == nil || !task.ScheduledAt.After(time.Now()) {
			s.redis.ZRem(ctx, sortedSetKey, taskID)
		}

		s.logger.Info(fmt.Sprintf("scheduled task pushed to queue: task_id=%s", taskID))
	}
}
