package controller

import (
	"io"

	"cnb.cool/mliev/push/message-push/app/sender"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
	"github.com/gin-gonic/gin"
)

// CallbackController 回调控制器
type CallbackController struct{}

// Handle 处理服务商回调（动态路由）
// POST/GET /api/callback/:provider
func (ctrl CallbackController) Handle(c *gin.Context, helper interfaces.HelperInterface) {
	providerCode := c.Param("provider")
	if providerCode == "" {
		c.JSON(200, gin.H{"code": 400, "message": "provider is required"})
		return
	}

	// 读取原始请求体
	rawBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(200, gin.H{"code": 400, "message": "failed to read request body"})
		return
	}

	// 构造回调请求
	req := &sender.CallbackRequest{
		ProviderCode: providerCode,
		RawBody:      rawBody,
		Headers:      make(map[string]string),
		QueryParams:  make(map[string]string),
	}

	// 收集请求头
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			req.Headers[key] = values[0]
		}
	}

	// 收集查询参数
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			req.QueryParams[key] = values[0]
		}
	}

	// 处理回调
	callbackService := service.NewCallbackService()
	if err := callbackService.HandleCallback(c.Request.Context(), providerCode, req); err != nil {
		helper.GetLogger().Error("callback handler error: " + err.Error())
		// 即使处理失败，也返回成功，避免服务商重复推送
		c.JSON(200, gin.H{"code": 0, "message": "ok"})
		return
	}

	// 返回成功（不同服务商可能期望不同的响应格式）
	c.JSON(200, gin.H{"code": 0, "message": "ok"})
}

// GetSupportedProviders 获取支持回调的服务商列表
// GET /api/callback/providers
func (ctrl CallbackController) GetSupportedProviders(c *gin.Context, helper interfaces.HelperInterface) {
	callbackService := service.NewCallbackService()
	providers := callbackService.GetSupportedProviders()
	SuccessWithData(c, gin.H{
		"providers": providers,
	})
}
