package queue

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Message 消息结构
type Message struct {
	ID     string
	TaskID string
	Data   map[string]interface{}
}

// Consumer 队列消费者
type Consumer struct {
	redis         *redis.Client
	streamName    string
	consumerGroup string
	consumerName  string
}

// NewConsumer 创建消费者
func NewConsumer(redisClient *redis.Client, consumerName string) *Consumer {
	return &Consumer{
		redis:         redisClient,
		streamName:    "push:stream:messages",
		consumerGroup: "push-workers",
		consumerName:  consumerName,
	}
}

// CreateGroup 创建消费者组
func (c *Consumer) CreateGroup(ctx context.Context) error {
	// 如果组不存在则创建，使用 MKSTREAM 选项
	err := c.redis.XGroupCreateMkStream(ctx, c.streamName, c.consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// ReadMessages 读取消息
func (c *Consumer) ReadMessages(ctx context.Context, count int64, blockTime time.Duration) ([]*Message, error) {
	// 从消费者组读取消息
	streams, err := c.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.consumerGroup,
		Consumer: c.consumerName,
		Streams:  []string{c.streamName, ">"},
		Count:    count,
		Block:    blockTime,
	}).Result()

	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var messages []*Message
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			message := &Message{
				ID:   msg.ID,
				Data: msg.Values,
			}

			if taskID, ok := msg.Values["task_id"].(string); ok {
				message.TaskID = taskID
			}

			messages = append(messages, message)
		}
	}

	return messages, nil
}

// Ack 确认消息
func (c *Consumer) Ack(ctx context.Context, messageID string) error {
	return c.redis.XAck(ctx, c.streamName, c.consumerGroup, messageID).Err()
}

// MoveToDeadLetter 移入死信队列
func (c *Consumer) MoveToDeadLetter(ctx context.Context, message *Message) error {
	// 推送到死信队列
	_, err := c.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: "push:stream:dead_letter",
		Values: message.Data,
	}).Result()

	return err
}

// ReclaimStale 用 XAUTOCLAIM 将组内闲置超过 minIdle 的 pending 消息认领到当前 consumer。
// consumer 名含 hostname-pid，实例崩溃/滚动重启后旧 consumer 不再被复用，
// 其 PEL 中未 Ack 的消息必须由存活实例定期回收，否则永久滞留。
func (c *Consumer) ReclaimStale(ctx context.Context, minIdle time.Duration, count int64) ([]*Message, error) {
	msgs, _, err := c.redis.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   c.streamName,
		Group:    c.consumerGroup,
		Consumer: c.consumerName,
		MinIdle:  minIdle,
		Start:    "0-0",
		Count:    count,
	}).Result()
	if err != nil {
		return nil, err
	}

	var messages []*Message
	for _, msg := range msgs {
		message := &Message{
			ID:   msg.ID,
			Data: msg.Values,
		}
		if taskID, ok := msg.Values["task_id"].(string); ok {
			message.TaskID = taskID
		}
		messages = append(messages, message)
	}
	return messages, nil
}

// CleanupIdleConsumers 删除组内无 pending 且闲置超过 minIdle 的 consumer 记录，
// 防止滚动重启产生的死 consumer 无限累积。
// XGROUP DELCONSUMER 会丢弃该 consumer 的 pending 消息，因此仅在 Pending==0 时删除；
// 存活 consumer 的阻塞读会持续刷新 idle，不会被误删。
func (c *Consumer) CleanupIdleConsumers(ctx context.Context, minIdle time.Duration) (int, error) {
	consumers, err := c.redis.XInfoConsumers(ctx, c.streamName, c.consumerGroup).Result()
	if err != nil {
		return 0, err
	}

	removed := 0
	for _, consumer := range consumers {
		if consumer.Name == c.consumerName || consumer.Pending > 0 || consumer.Idle < minIdle {
			continue
		}
		if err := c.redis.XGroupDelConsumer(ctx, c.streamName, c.consumerGroup, consumer.Name).Err(); err == nil {
			removed++
		}
	}
	return removed, nil
}
