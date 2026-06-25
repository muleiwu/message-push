// Package sender 是 sender 限界上下文的对外门面（facade）。
//
// 它通过类型别名复用领域层类型，保持历史 API 表面（旧 app/sender.* 调用方几乎零改动），
// 并提供 GetResolver() 从 go-web DI 容器解析发送器解析器，
// 供其他模块以接口方式调用，实现“独立模块调用”。
package sender

import (
	"cnb.cool/mliev/open/go-web/pkg/container"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

// ==================== 端口与解析器 ====================

type (
	Sender          = domain.Sender
	BatchSender     = domain.BatchSender
	CallbackHandler = domain.CallbackHandler
	StatusQuerier   = domain.StatusQuerier
	StatusPuller    = domain.StatusPuller
	Resolver        = domain.Resolver
)

// ==================== 值对象 ====================

type (
	SendRequest         = domain.SendRequest
	SendResponse        = domain.SendResponse
	BatchSendRequest    = domain.BatchSendRequest
	BatchSendResponse   = domain.BatchSendResponse
	CallbackRequest     = domain.CallbackRequest
	CallbackResult      = domain.CallbackResult
	CallbackResponse    = domain.CallbackResponse
	StatusQueryRequest  = domain.StatusQueryRequest
	StatusQueryResult   = domain.StatusQueryResult
	StatusQueryResponse = domain.StatusQueryResponse
	StatusPullRequest   = domain.StatusPullRequest
)

// ==================== 服务商注册表 ====================

type (
	ProviderMeta = domain.ProviderMeta
	ConfigField  = domain.ConfigField
	FieldOption  = domain.FieldOption
)

// ==================== 批量与默认配置常量 ====================

const (
	MaxBatchSizeTencentSMS = domain.MaxBatchSizeTencentSMS
	MaxBatchSizeAliyunSMS  = domain.MaxBatchSizeAliyunSMS
	DefaultMaxRetry        = domain.DefaultMaxRetry
	DefaultRetryInterval   = domain.DefaultRetryInterval
	DefaultTimeout         = domain.DefaultTimeout
)

// GetResolver 从 DI 容器解析发送器解析器（domain.Resolver）。
// 由 modules/sender/assembly 在装配阶段注册到容器。
func GetResolver() domain.Resolver {
	return container.MustGet[domain.Resolver]()
}
