package middleware

import (
	"strconv"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dao"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	appDao := dao.NewApplicationDAO()

	return func(c *gin.Context) {
		// 从Header获取AppID和签名信息
		appID := c.GetHeader("X-App-Id")
		signature := c.GetHeader("X-Signature")
		timestampStr := c.GetHeader("X-Timestamp")

		if appID == "" || signature == "" || timestampStr == "" {
			controller.FailWithCode(c, constants.CodeUnauthorized)
			c.Abort()
			return
		}

		// 验证时间戳
		timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			controller.FailWithCode(c, constants.CodeInvalidTimestamp)
			c.Abort()
			return
		}

		// 验证应用是否存在
		app, err := appDao.GetByAppID(appID)
		if err != nil {
			controller.FailWithCode(c, constants.CodeInvalidAppID)
			c.Abort()
			return
		}

		// 检查应用状态
		if app.Status != 1 {
			controller.FailWithCode(c, constants.CodeAppDisabled)
			c.Abort()
			return
		}

		// 将应用信息存入上下文
		c.Set("app_id", app.AppID)
		c.Set("app_db_id", app.ID)
		c.Set("signature", signature)
		c.Set("timestamp", timestamp)

		c.Next()
	}
}

// OptionalAuthMiddleware 可选认证中间件（不强制要求认证）
func OptionalAuthMiddleware() gin.HandlerFunc {
	appDao := dao.NewApplicationDAO()

	return func(c *gin.Context) {
		appID := c.GetHeader("X-App-Id")
		if appID == "" {
			c.Next()
			return
		}

		// 如果提供了AppID，则验证
		app, err := appDao.GetByAppID(appID)
		if err == nil && app.Status == 1 {
			c.Set("app_id", app.AppID)
			c.Set("app_db_id", app.ID)
		}

		c.Next()
	}
}
