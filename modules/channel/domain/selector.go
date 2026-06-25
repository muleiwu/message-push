// Package domain 是 channel 限界上下文的领域层：
// 定义通道选择端口（Selector）与通道节点值对象（ChannelNode）。
package domain

import (
	"context"

	"cnb.cool/mliev/push/message-push/app/model"
)

// ChannelNode 通道节点（带权重）
type ChannelNode struct {
	ChannelTemplateBinding *model.ChannelTemplateBinding // 通道模板绑定配置
	ProviderAccount        *model.ProviderAccount        // 服务商账号配置
	CurrentWeight          int                           // 当前权重
	EffectiveWeight        int                           // 有效权重
}

// Selector 通道选择端口：在通道下按优先级分组 + 平滑加权轮询挑选服务商节点，
// 并提供权重重置与缓存失效能力。基础设施层提供实现（Redis 权重状态 + 绑定缓存），
// 经 assembly 注册到 go-web 容器，供 messaging / delivery 等模块以接口方式调用。
type Selector interface {
	// Select 选择通道节点（平滑加权轮询）
	Select(ctx context.Context, channelID uint, messageType string, appID string, receiver string) (*ChannelNode, error)
	// SelectWithExcludes 选择通道节点，支持排除指定服务商（规则引擎切换时使用）
	SelectWithExcludes(ctx context.Context, channelID uint, messageType string, appID string, receiver string, excludeProviderIDs []uint) (*ChannelNode, error)

	// ReportSuccess 报告发送成功（供熔断/自动恢复使用）
	ReportSuccess(providerAccountID uint)
	// ReportFailure 报告发送失败（供熔断/自动禁用使用）
	ReportFailure(providerAccountID uint)

	// ResetWeightsByChannelID 重置指定通道的权重状态
	ResetWeightsByChannelID(channelID uint)
	// ClearCache 清除所有缓存
	ClearCache()
	// ClearCacheByChannelID 清除指定通道的缓存
	ClearCacheByChannelID(channelID uint)
	// ClearCacheByKey 清除指定 key 的缓存
	ClearCacheByKey(channelID uint, messageType string)
	// InvalidateCacheForBinding 绑定配置变更后清除相关缓存
	InvalidateCacheForBinding(channelID uint)
}
