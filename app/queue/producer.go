package queue

import (
	"context"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/redis/go-redis/v9"
)

// Producer 队列生产者
type Producer struct {
	redis      *redis.Client
	streamName string
}

// NewProducer 创建生产者
func NewProducer(redisClient *redis.Client) *Producer {
	return &Producer{
		redis:      redisClient,
		streamName: "push:stream:messages",
	}
}

// Push 推送任务到队列
func (p *Producer) Push(ctx context.Context, task *model.PushTask) error {
	// 检查是否是定时任务
	if task.ScheduledAt != nil && task.ScheduledAt.After(time.Now()) {
		return p.pushScheduled(ctx, task)
	}

	// 立即推送到Stream
	data := map[string]interface{}{
		"task_id":         task.TaskID,
		"app_id":          task.AppID,
		"push_channel_id": task.PushChannelID,
		"receiver":        task.Receiver,
		"template_code":   task.TemplateCode,
		"template_params": task.TemplateParams,
		"signature":       task.Signature,
		"retry_count":     task.RetryCount,
		"max_retry":       task.MaxRetry,
	}

	_, err := p.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: p.streamName,
		Values: data,
	}).Result()

	return err
}

// pushScheduled 推送定时任务
func (p *Producer) pushScheduled(ctx context.Context, task *model.PushTask) error {
	// 推送到Sorted Set，score为定时发送时间
	score := float64(task.ScheduledAt.Unix())

	return p.redis.ZAdd(ctx, "push:scheduled:tasks", redis.Z{
		Score:  score,
		Member: task.TaskID,
	}).Err()
}

// PushBatch 批量推送任务
func (p *Producer) PushBatch(ctx context.Context, tasks []*model.PushTask) error {
	pipe := p.redis.Pipeline()

	for _, task := range tasks {
		data := map[string]interface{}{
			"task_id":         task.TaskID,
			"app_id":          task.AppID,
			"push_channel_id": task.PushChannelID,
			"receiver":        task.Receiver,
			"template_code":   task.TemplateCode,
			"template_params": task.TemplateParams,
			"signature":       task.Signature,
			"retry_count":     task.RetryCount,
			"max_retry":       task.MaxRetry,
		}

		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: p.streamName,
			Values: data,
		})
	}

	_, err := pipe.Exec(ctx)
	return err
}
