package worker

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/queue"
	"github.com/muleiwu/gsr"
	"github.com/redis/go-redis/v9"
)

// MessageHandlerFunc 消息处理函数
type MessageHandlerFunc func(ctx context.Context, message *queue.Message) error

// instanceID 实例唯一标识（hostname-pid），K8s 下 hostname 即 Pod 名。
// 多实例部署时保证 consumer 名在 consumer group 内不重名，避免 PEL 归属混淆。
var instanceID = func() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}()

// Worker 工作者
type Worker struct {
	id       int
	consumer *queue.Consumer
	handler  MessageHandlerFunc
	stopCh   chan struct{}
	wg       *sync.WaitGroup
	logger   gsr.Logger
}

// NewWorker 创建工作者
func NewWorker(id int, redisClient *redis.Client, handler MessageHandlerFunc) *Worker {
	consumerName := fmt.Sprintf("worker-%s-%d", instanceID, id)
	return &Worker{
		id:       id,
		consumer: queue.NewConsumer(redisClient, consumerName),
		handler:  handler,
		stopCh:   make(chan struct{}),
		wg:       &sync.WaitGroup{},
		logger:   helper.GetLogger(),
	}
}

// Start 启动工作者
func (w *Worker) Start(ctx context.Context) error {
	// 确保消费者组存在
	if err := w.consumer.CreateGroup(ctx); err != nil {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	w.wg.Add(1)
	go w.run(ctx)

	w.logger.Info(fmt.Sprintf("worker started id=%d", w.id))
	return nil
}

// Stop 停止工作者
func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	w.logger.Info(fmt.Sprintf("worker stopped id=%d", w.id))
}

// run 运行工作循环
func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()

	for {
		select {
		case <-w.stopCh:
			return
		default:
			w.processMessages(ctx)
		}
	}
}

// processMessages 处理消息
func (w *Worker) processMessages(ctx context.Context) {
	// 读取消息，阻塞5秒
	messages, err := w.consumer.ReadMessages(ctx, 10, 5*time.Second)
	if err != nil {
		w.logger.Error(fmt.Sprintf("failed to read messages worker_id=%d err=%v", w.id, err))
		return
	}

	// 处理每条消息
	for _, msg := range messages {
		if err := w.handleMessage(ctx, msg); err != nil {
			w.logger.Error(fmt.Sprintf("failed to handle message worker_id=%d message_id=%s task_id=%s err=%v", w.id, msg.ID, msg.TaskID, err))
		}
	}
}

// handleMessage 处理单条消息
func (w *Worker) handleMessage(ctx context.Context, msg *queue.Message) error {
	return handleQueueMessage(ctx, w.consumer, w.handler, w.logger, fmt.Sprintf("worker_id=%d", w.id), msg)
}

// handleQueueMessage 处理单条消息：失败移入死信队列；无论成败均 Ack，避免重复处理
func handleQueueMessage(ctx context.Context, consumer *queue.Consumer, handler MessageHandlerFunc, logger gsr.Logger, label string, msg *queue.Message) error {
	logger.Info(fmt.Sprintf("processing message %s message_id=%s task_id=%s", label, msg.ID, msg.TaskID))

	// 执行业务逻辑
	if err := handler(ctx, msg); err != nil {
		logger.Error(fmt.Sprintf("handler error %s message_id=%s err=%v", label, msg.ID, err))

		// 移入死信队列
		if dlErr := consumer.MoveToDeadLetter(ctx, msg); dlErr != nil {
			logger.Error(fmt.Sprintf("failed to move to dead letter %s message_id=%s err=%v", label, msg.ID, dlErr))
		}

		// 即使失败也要Ack，避免重复处理
		return consumer.Ack(ctx, msg.ID)
	}

	// 确认消息
	return consumer.Ack(ctx, msg.ID)
}

// WorkerPool 工作者池
type WorkerPool struct {
	workers   []*Worker
	reclaimer *Reclaimer
	logger    gsr.Logger
}

// NewWorkerPool 创建工作者池
func NewWorkerPool(size int, redisClient *redis.Client, handler MessageHandlerFunc) *WorkerPool {
	workers := make([]*Worker, size)
	for i := 0; i < size; i++ {
		workers[i] = NewWorker(i+1, redisClient, handler)
	}

	return &WorkerPool{
		workers:   workers,
		reclaimer: NewReclaimer(redisClient, handler),
		logger:    helper.GetLogger(),
	}
}

// Start 启动所有工作者和孤儿消息回收器
func (p *WorkerPool) Start(ctx context.Context) error {
	for _, worker := range p.workers {
		if err := worker.Start(ctx); err != nil {
			return err
		}
	}

	p.reclaimer.Start(ctx)

	p.logger.Info(fmt.Sprintf("worker pool started size=%d", len(p.workers)))
	return nil
}

// Stop 停止所有工作者和回收器
func (p *WorkerPool) Stop() {
	var wg sync.WaitGroup
	for _, worker := range p.workers {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			w.Stop()
		}(worker)
	}

	wg.Wait()
	p.reclaimer.Stop()
	p.logger.Info("worker pool stopped")
}
