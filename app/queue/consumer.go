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

// GetPendingMessages 获取待处理的消息（超时未确认）
func (c *Consumer) GetPendingMessages(ctx context.Context) ([]string, error) {
	// 获取pending消息列表
	pending, err := c.redis.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: c.streamName,
		Group:  c.consumerGroup,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()

	if err != nil {
		return nil, err
	}

	var messageIDs []string
	for _, p := range pending {
		// 如果消息超过5分钟未确认，则重新处理
		if p.Idle > 5*time.Minute {
			messageIDs = append(messageIDs, p.ID)
		}
	}

	return messageIDs, nil
}
