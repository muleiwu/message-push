// Package domain 是 delivery 限界上下文的领域层：
// 定义消息投递的生产者端口（Producer）——将推送任务入队（即时流 / 定时有序集合）。
package domain

import (
	"context"

	"cnb.cool/mliev/push/message-push/app/model"
)

// Producer 消息生产者端口：将推送任务写入投递队列。
// 即时任务进入 Redis Stream，定时任务进入有序集合，由 delivery 的 worker / 调度器消费。
// 基础设施层提供实现，经 assembly 注册到 go-web 容器，供 messaging / ruleengine 等模块调用。
type Producer interface {
	// Push 推送单个任务到队列
	Push(ctx context.Context, task *model.PushTask) error
	// PushBatch 批量推送任务到队列
	PushBatch(ctx context.Context, tasks []*model.PushTask) error
}
