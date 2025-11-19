package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cnb.cool/mliev/push/message-push/app/queue"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
	"github.com/redis/go-redis/v9"
)

// MessageHandlerFunc 消息处理函数
type MessageHandlerFunc func(ctx context.Context, message *queue.Message) error

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
	consumerName := fmt.Sprintf("worker-%d", id)
	return &Worker{
		id:       id,
		consumer: queue.NewConsumer(redisClient, consumerName),
		handler:  handler,
		stopCh:   make(chan struct{}),
		wg:       &sync.WaitGroup{},
		logger:   helper.GetHelper().GetLogger(),
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
	w.logger.Info(fmt.Sprintf("processing message worker_id=%d message_id=%s task_id=%s", w.id, msg.ID, msg.TaskID))

	// 执行业务逻辑
	if err := w.handler(ctx, msg); err != nil {
		w.logger.Error(fmt.Sprintf("handler error worker_id=%d message_id=%s err=%v", w.id, msg.ID, err))

		// 移入死信队列
		if dlErr := w.consumer.MoveToDeadLetter(ctx, msg); dlErr != nil {
			w.logger.Error(fmt.Sprintf("failed to move to dead letter worker_id=%d message_id=%s err=%v", w.id, msg.ID, dlErr))
		}

		// 即使失败也要Ack，避免重复处理
		return w.consumer.Ack(ctx, msg.ID)
	}

	// 确认消息
	return w.consumer.Ack(ctx, msg.ID)
}

// WorkerPool 工作者池
type WorkerPool struct {
	workers []*Worker
	logger  gsr.Logger
}

// NewWorkerPool 创建工作者池
func NewWorkerPool(size int, redisClient *redis.Client, handler MessageHandlerFunc) *WorkerPool {
	workers := make([]*Worker, size)
	for i := 0; i < size; i++ {
		workers[i] = NewWorker(i+1, redisClient, handler)
	}

	return &WorkerPool{
		workers: workers,
		logger:  helper.GetHelper().GetLogger(),
	}
}

// Start 启动所有工作者
func (p *WorkerPool) Start(ctx context.Context) error {
	for _, worker := range p.workers {
		if err := worker.Start(ctx); err != nil {
			return err
		}
	}

	p.logger.Info(fmt.Sprintf("worker pool started size=%d", len(p.workers)))
	return nil
}

// Stop 停止所有工作者
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
	p.logger.Info("worker pool stopped")
}
