// Package domain 是 ruleengine 限界上下文的领域层：
// 定义失败处理规则引擎端口（RuleEngine）与评估请求/结果值对象。
package domain

import (
	"context"

	"cnb.cool/mliev/push/message-push/app/model"
)

// EvaluateRequest 规则评估请求
type EvaluateRequest struct {
	Scene        string          // 场景：send_failure / callback_failure
	ProviderCode string          // 供应商代码
	MessageType  string          // 消息类型
	ErrorCode    string          // 错误码
	ErrorMessage string          // 错误消息
	Task         *model.PushTask // 任务信息
}

// EvaluateResult 规则评估结果
type EvaluateResult struct {
	Action      string             // 动作：retry, switch_provider, fail, alert
	MatchedRule *model.FailureRule // 匹配到的规则
	HasMatch    bool               // 是否匹配到规则
}

// RuleEngine 失败处理规则引擎端口：根据场景/供应商/错误码/关键字匹配规则，
// 返回推荐动作（重试/切换供应商/失败/告警），并支持刷新内存规则缓存。
// 基础设施层提供实现（DB 加载 + 内存缓存），经 assembly 注册到 go-web 容器。
type RuleEngine interface {
	// Evaluate 评估失败并返回推荐动作
	Evaluate(ctx context.Context, req *EvaluateRequest) *EvaluateResult
	// RefreshCache 刷新规则缓存（管理端增删改后调用）
	RefreshCache()
}
