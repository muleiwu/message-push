package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/helper"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware 认证中间件
func AuthMiddleware() gin.HandlerFunc {
	appDao := dao.NewApplicationDAO()
	signatureHelper := helper.NewSignatureHelper()

	return func(c *gin.Context) {
		// 从Header获取AppID和签名信息
		appID := c.GetHeader("X-App-Id")
		signature := c.GetHeader("X-Signature")
		timestampStr := c.GetHeader("X-Timestamp")
		nonce := c.GetHeader("X-Nonce")

		if appID == "" || signature == "" || timestampStr == "" || nonce == "" {
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

		// 解密 app secret
		appSecret, err := helper.DecryptAppSecret(app.AppSecret)
		if err != nil {
			controller.FailWithCode(c, constants.CodeUnauthorized)
			c.Abort()
			return
		}

		// 读取请求体用于签名验证
		var bodyParams map[string]interface{}
		if c.Request.Body != nil {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err != nil {
				controller.FailWithCode(c, constants.CodeUnauthorized)
				c.Abort()
				return
			}
			// 将 body 放回，以便后续 handler 可以读取
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			// 解析 body 为 map
			if len(bodyBytes) > 0 {
				if err := json.Unmarshal(bodyBytes, &bodyParams); err != nil {
					controller.FailWithMessage(c, "invalid request body")
					c.Abort()
					return
				}
			}
		}

		// 验证签名
		method := c.Request.Method
		path := c.Request.URL.Path
		if !signatureHelper.VerifySignature(appSecret, method, path, bodyParams, timestamp, nonce, signature) {
			controller.FailWithCode(c, constants.CodeInvalidSignature)
			c.Abort()
			return
		}

		// 将应用信息存入上下文
		c.Set("app_id", app.AppID)
		c.Set("app_db_id", app.ID)

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
