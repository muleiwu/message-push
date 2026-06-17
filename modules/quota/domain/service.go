// Package domain 是 quota 限界上下文的领域层：
// 定义按应用的每日配额端口（Service），基于 Redis 计数。
package domain

import "context"

// Service 配额服务端口：基于 Redis 的按应用每日配额校验、计数与用量查询。
// 基础设施层提供实现，经 assembly 注册到 go-web 容器。
type Service interface {
	// Check 原子校验并计数：未超过 dailyLimit 时计数 +1 并返回 true，超限返回 false。
	Check(ctx context.Context, appID uint, dailyLimit int) (bool, error)
	// Increment 手动增加配额计数（发送后补偿场景）。
	Increment(ctx context.Context, appID uint, count int) error
	// GetUsage 返回今日已用量与配额上限。
	GetUsage(ctx context.Context, appID uint) (used int64, limit int64, err error)
}
