package helper

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetBaseURL 获取基础URL，支持反向代理场景
// 优先级：X-Forwarded headers > 请求 headers
// 参数：
//   - c: gin.Context
//   - path: 路径（如 "/api/callback"）
//
// 返回：完整的 URL（如 "https://example.com/api/callback"）
func GetBaseURL(c *gin.Context, path string) string {
	// 1. 检查反向代理 headers
	forwardedProto := c.GetHeader("X-Forwarded-Proto")
	forwardedHost := c.GetHeader("X-Forwarded-Host")

	if forwardedProto != "" && forwardedHost != "" {
		return strings.TrimSuffix(fmt.Sprintf("%s://%s%s", forwardedProto, forwardedHost, path), "/")
	}

	// 2. 回退到请求信息
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}

	// 如果有 X-Forwarded-Proto 但没有 X-Forwarded-Host
	if forwardedProto != "" {
		scheme = forwardedProto
	}

	host := c.Request.Host
	if forwardedHost != "" {
		host = forwardedHost
	}

	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}
