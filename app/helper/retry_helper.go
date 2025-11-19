package helper

import (
	"strings"
	"time"
)

// RetryHelper 重试助手
type RetryHelper struct {
	initialDelay time.Duration
	maxDelay     time.Duration
}

// NewRetryHelper 创建重试助手
func NewRetryHelper() *RetryHelper {
	return &RetryHelper{
		initialDelay: 2 * time.Second,
		maxDelay:     60 * time.Second,
	}
}

// ShouldRetry 判断是否应该重试
func (h *RetryHelper) ShouldRetry(errorMsg string, retryCount, maxRetry int) (bool, time.Duration) {
	if retryCount >= maxRetry {
		return false, 0
	}

	if h.isBusinessError(errorMsg) {
		return false, 0
	}

	delay := h.calculateBackoff(retryCount)
	return true, delay
}

// isBusinessError 判断是否为业务错误（不应重试）
func (h *RetryHelper) isBusinessError(errorMsg string) bool {
	nonRetryableErrors := []string{
		"invalid parameter", "参数错误",
		"手机号无效", "invalid phone number",
		"余额不足", "insufficient balance",
		"签名未审核", "signature not approved",
		"模板未审核", "template not approved",
		"黑名单", "blacklist",
	}

	errorLower := strings.ToLower(errorMsg)
	for _, keyword := range nonRetryableErrors {
		if strings.Contains(errorLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// calculateBackoff 计算指数退避延迟
func (h *RetryHelper) calculateBackoff(retryCount int) time.Duration {
	delay := h.initialDelay
	for i := 1; i < retryCount; i++ {
		delay *= 2
		if delay > h.maxDelay {
			return h.maxDelay
		}
	}
	return delay
}
