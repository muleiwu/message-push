package constants

// 错误码定义
const (
	// 0xxxx - 成功
	CodeSuccess = 0

	// 1xxxx - 请求错误
	CodeBadRequest            = 10001 // 请求参数错误
	CodeInvalidJSON           = 10002 // JSON格式错误
	CodeMissingParameter      = 10003 // 缺少必填参数
	CodeInvalidParameter      = 10004 // 参数值非法
	CodeInvalidReceiver       = 10005 // 接收者格式错误
	CodeInvalidTemplateParams = 10006 // 模板参数错误

	// 2xxxx - 鉴权错误
	CodeUnauthorized     = 20001 // 未授权
	CodeInvalidAppID     = 20002 // 无效的AppID
	CodeInvalidSignature = 20003 // 签名验证失败
	CodeInvalidTimestamp = 20004 // 时间戳无效（防重放）
	CodeIPNotAllowed     = 20005 // IP不在白名单
	CodeAppDisabled      = 20006 // 应用已禁用

	// 3xxxx - 业务错误
	CodeRateLimitExceeded  = 30001 // 超出速率限制
	CodeQuotaExceeded      = 30002 // 超出配额限制
	CodeChannelNotFound    = 30003 // 推送通道不存在
	CodeChannelDisabled    = 30004 // 推送通道已禁用
	CodeTemplateNotFound   = 30005 // 模板不存在
	CodeNoAvailableChannel = 30006 // 无可用通道
	CodeTaskNotFound       = 30007 // 任务不存在
	CodeBatchNotFound      = 30008 // 批量任务不存在

	// 4xxxx - 系统错误
	CodeInternalError  = 40001 // 内部错误
	CodeDatabaseError  = 40002 // 数据库错误
	CodeRedisError     = 40003 // Redis错误
	CodeQueueError     = 40004 // 队列错误
	CodeProviderError  = 40005 // 服务商错误
	CodeNetworkTimeout = 40006 // 网络超时
	CodeCircuitOpen    = 40007 // 熔断器打开
)

// ErrorMessages 错误信息映射
var ErrorMessages = map[int]string{
	CodeSuccess:               "success",
	CodeBadRequest:            "bad request",
	CodeInvalidJSON:           "invalid json format",
	CodeMissingParameter:      "missing required parameter",
	CodeInvalidParameter:      "invalid parameter value",
	CodeInvalidReceiver:       "invalid receiver format",
	CodeInvalidTemplateParams: "invalid template parameters",
	CodeUnauthorized:          "unauthorized",
	CodeInvalidAppID:          "invalid app id",
	CodeInvalidSignature:      "invalid signature",
	CodeInvalidTimestamp:      "invalid timestamp",
	CodeIPNotAllowed:          "ip not allowed",
	CodeAppDisabled:           "app disabled",
	CodeRateLimitExceeded:     "rate limit exceeded",
	CodeQuotaExceeded:         "quota exceeded",
	CodeChannelNotFound:       "channel not found",
	CodeChannelDisabled:       "channel disabled",
	CodeTemplateNotFound:      "template not found",
	CodeNoAvailableChannel:    "no available channel",
	CodeTaskNotFound:          "task not found",
	CodeBatchNotFound:         "batch not found",
	CodeInternalError:         "internal server error",
	CodeDatabaseError:         "database error",
	CodeRedisError:            "redis error",
	CodeQueueError:            "queue error",
	CodeProviderError:         "provider error",
	CodeNetworkTimeout:        "network timeout",
	CodeCircuitOpen:           "circuit breaker open",
}

// GetErrorMessage 获取错误信息
func GetErrorMessage(code int) string {
	if msg, exists := ErrorMessages[code]; exists {
		return msg
	}
	return "unknown error"
}
