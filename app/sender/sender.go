package sender

import (
	"context"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
)

// ==================== 批量限制常量 ====================

const (
	MaxBatchSizeTencentSMS = 200  // 腾讯云短信批量上限
	MaxBatchSizeAliyunSMS  = 1000 // 阿里云短信批量上限
)

// ==================== 默认通道配置 ====================

const (
	DefaultMaxRetry      = 3  // 默认最大重试次数
	DefaultRetryInterval = 2  // 默认重试间隔（秒）
	DefaultTimeout       = 10 // 默认超时时间（秒）
)

// ==================== 发送相关 ====================

// SendRequest 发送请求
type SendRequest struct {
	Task                   *model.PushTask
	ProviderAccount        *model.ProviderAccount        // 服务商账号配置
	ChannelTemplateBinding *model.ChannelTemplateBinding // 通道模板绑定配置
	Signature              *model.ProviderSignature      // 签名配置（用于SMS类型）
}

// SendResponse 发送响应
type SendResponse struct {
	Success      bool
	ProviderID   string // 服务商返回的消息ID
	ErrorCode    string
	ErrorMessage string
	TaskID       string // 批量发送时用于关联；单发时可忽略
	Status       string // 任务状态：processing(等待回调) 或 success(直接成功)
	RequestData  string // 发送给供应商的请求参数（JSON格式），用于调试
	ResponseData string // 供应商返回的响应数据（JSON格式），用于调试
}

// Sender 发送器接口
type Sender interface {
	// Send 发送消息
	// ctx 用于控制超时和取消，实现者应检查 ctx.Done()
	Send(ctx context.Context, req *SendRequest) (*SendResponse, error)

	// GetProviderCode 获取服务商代码（唯一标识）
	GetProviderCode() string
}

// ==================== 批量发送相关 ====================

// BatchSendRequest 批量发送请求
// 注意：不同服务商有不同的批量限制，具体参见 MaxBatchSize* 常量
type BatchSendRequest struct {
	Tasks                  []*model.PushTask
	ProviderAccount        *model.ProviderAccount
	ChannelTemplateBinding *model.ChannelTemplateBinding
	Signature              *model.ProviderSignature
}

// BatchSendResponse 批量发送响应
type BatchSendResponse struct {
	Results []*SendResponse // 每个任务的发送结果
}

// BatchSender 批量发送器接口（可选实现）
type BatchSender interface {
	Sender
	// BatchSend 批量发送消息
	BatchSend(ctx context.Context, req *BatchSendRequest) (*BatchSendResponse, error)
	// SupportsBatchSend 是否支持批量发送
	SupportsBatchSend() bool
}

// ==================== 回调处理相关 ====================

// CallbackRequest 回调请求
type CallbackRequest struct {
	ProviderCode string            // 服务商代码
	RawBody      []byte            // 原始请求体
	Headers      map[string]string // 请求头（用于签名验证等）
	QueryParams  map[string]string // URL 查询参数
	FormData     map[string]string // 表单数据（用于 form-data 请求）
}

// CallbackResult 回调结果
// TaskID 由上层服务通过 ProviderID 反查获取
type CallbackResult struct {
	ProviderID   string    // 服务商消息ID
	Status       string    // 状态：使用 constants.CallbackStatus* 常量
	ErrorCode    string    // 错误码
	ErrorMessage string    // 错误信息
	ReportTime   time.Time // 回调时间
}

// CallbackResponse 回调响应（返回给服务商）
type CallbackResponse struct {
	StatusCode int    // HTTP 状态码
	Body       string // 响应体（字符串格式）
}

// CallbackHandler 回调处理器接口
type CallbackHandler interface {
	// HandleCallback 处理服务商回调
	// 返回值：响应信息（实体，始终返回）、回调结果列表、错误
	HandleCallback(ctx context.Context, req *CallbackRequest) (CallbackResponse, []*CallbackResult, error)
	// GetProviderCode 获取服务商代码
	GetProviderCode() string
	// SupportsCallback 是否支持回调
	SupportsCallback() bool
}

// ==================== 状态查询相关 ====================

// StatusQueryRequest 单条状态查询请求（阿里云、腾讯云）
type StatusQueryRequest struct {
	ProviderAccount *model.ProviderAccount
	ProviderMsgID   string    // 服务商消息ID
	PhoneNumber     string    // 手机号
	SendDate        time.Time // 发送日期
}

// StatusQueryResult 状态查询结果
type StatusQueryResult struct {
	ProviderMsgID string    // 服务商消息ID
	PhoneNumber   string    // 手机号
	Status        string    // 状态：使用 constants.CallbackStatus* 常量
	ErrorCode     string    // 错误码
	ErrorMessage  string    // 错误信息
	ReportTime    time.Time // 状态报告时间
}

// StatusQueryResponse 状态查询响应
type StatusQueryResponse struct {
	Results []*StatusQueryResult
}

// StatusQuerier 单条状态查询接口（阿里云、腾讯云）
// 适用于支持按消息ID或手机号查询状态的服务商
type StatusQuerier interface {
	// QueryStatus 查询消息状态
	QueryStatus(ctx context.Context, req *StatusQueryRequest) (*StatusQueryResponse, error)
	// SupportsStatusQuery 是否支持状态查询
	SupportsStatusQuery() bool
	// GetProviderCode 获取服务商代码
	GetProviderCode() string
}

// StatusPullRequest 批量拉取请求（掌榕网）
type StatusPullRequest struct {
	ProviderAccount *model.ProviderAccount
}

// StatusPuller 批量状态拉取接口（掌榕网）
// 适用于只支持批量拉取待处理状态的服务商
type StatusPuller interface {
	// PullStatus 批量拉取待处理状态
	// 注意：已拉取的状态不会再次返回
	PullStatus(ctx context.Context, req *StatusPullRequest) (*StatusQueryResponse, error)
	// SupportsStatusPull 是否支持状态拉取
	SupportsStatusPull() bool
	// GetProviderCode 获取服务商代码
	GetProviderCode() string
}
