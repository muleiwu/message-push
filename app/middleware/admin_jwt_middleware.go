package middleware

import (
	"strings"

	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/helper"
	"github.com/gin-gonic/gin"
)

// AdminJWTMiddleware 管理员 JWT 认证中间件
func AdminJWTMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 Header 获取 Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			controller.ErrorResponse(c, 401, "未提供认证令牌")
			c.Abort()
			return
		}

		// 检查格式是否为 Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			controller.ErrorResponse(c, 401, "认证令牌格式错误")
			c.Abort()
			return
		}

		tokenString := parts[1]

		// 解析 token
		claims, err := helper.ParseToken(tokenString)
		if err != nil {
			controller.ErrorResponse(c, 401, "无效的认证令牌")
			c.Abort()
			return
		}

		// 将用户信息存入上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		c.Next()
	}
}
