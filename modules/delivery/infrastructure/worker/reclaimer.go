package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/queue"
	"github.com/muleiwu/gsr"
	"github.com/redis/go-redis/v9"
)

const (
	// reclaimInterval 回收扫描间隔
	reclaimInterval = time.Minute
	// reclaimMinIdle 消息闲置超过该时长视为孤儿（持有者已崩溃），可被回收重新处理；
	// 须远大于单条消息的正常处理耗时，避免抢走存活 worker 手中的慢消息
	reclaimMinIdle = 5 * time.Minute
	// consumerRetention consumer 闲置超过该时长且无 pending 时删除记录，防止滚动重启后无限累积
	consumerRetention = 30 * time.Minute
	// reclaimBatchSize 单次 XAUTOCLAIM 认领的消息数
	reclaimBatchSize = 100
)

// Reclaimer 孤儿消息回收器。
// consumer 名含 hostname-pid，实例崩溃/滚动重启后旧 consumer 不再被复用，
// 其 PEL 中未 Ack 的消息由回收器定期 XAUTOCLAIM 接管并重新处理
// （消费侧有 CAS 状态抢占，已处理过的任务会被跳过，不会重复下发）。
// 多实例同时运行安全：XAUTOCLAIM 原子认领，每条消息只会被一个实例接管。
type Reclaimer struct {
	consumer *queue.Consumer
	handler  MessageHandlerFunc
	stopCh   chan struct{}
	wg       sync.WaitGroup
	logger   gsr.Logger
}

// NewReclaimer 创建回收器
func NewReclaimer(redisClient *redis.Client, handler MessageHandlerFunc) *Reclaimer {
	return &Reclaimer{
		consumer: queue.NewConsumer(redisClient, "reclaimer-"+instanceID),
		handler:  handler,
		stopCh:   make(chan struct{}),
		logger:   helper.GetLogger(),
	}
}

// Start 启动回收器
func (r *Reclaimer) Start(ctx context.Context) {
	r.wg.Add(1)
	go r.run(ctx)
	r.logger.Info("message reclaimer started")
}

// Stop 停止回收器
func (r *Reclaimer) Stop() {
	close(r.stopCh)
	r.wg.Wait()
	r.logger.Info("message reclaimer stopped")
}

// run 运行回收循环
func (r *Reclaimer) run(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(reclaimInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.reclaim(ctx)
		case <-r.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// reclaim 回收并处理孤儿消息，随后清理死 consumer 记录
func (r *Reclaimer) reclaim(ctx context.Context) {
	for {
		// 认领后消息归属当前 consumer 且 idle 归零，重复从 0-0 认领不会拿到同一批
		messages, err := r.consumer.ReclaimStale(ctx, reclaimMinIdle, reclaimBatchSize)
		if err != nil {
			r.logger.Error(fmt.Sprintf("failed to reclaim stale messages: %v", err))
			break
		}
		if len(messages) == 0 {
			break
		}

		r.logger.Info(fmt.Sprintf("reclaimed %d orphan messages from dead consumers", len(messages)))

		for _, msg := range messages {
			if err := handleQueueMessage(ctx, r.consumer, r.handler, r.logger, "reclaimer", msg); err != nil {
				r.logger.Error(fmt.Sprintf("failed to handle reclaimed message message_id=%s task_id=%s err=%v", msg.ID, msg.TaskID, err))
			}
		}

		if len(messages) < reclaimBatchSize {
			break
		}
	}

	if removed, err := r.consumer.CleanupIdleConsumers(ctx, consumerRetention); err != nil {
		r.logger.Warn(fmt.Sprintf("failed to cleanup idle consumers: %v", err))
	} else if removed > 0 {
		r.logger.Info(fmt.Sprintf("removed %d idle consumers from group", removed))
	}
}
