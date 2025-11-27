package controller

import (
	"io"
	"strconv"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/sender"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
	"github.com/gin-gonic/gin"
)

// CallbackController 回调控制器
type CallbackController struct{}

// Handle 处理服务商回调（动态路由）
// POST/GET /api/callback/:id
func (ctrl CallbackController) Handle(c *gin.Context, helper interfaces.HelperInterface) {
	idStr := c.Param("id")
	if idStr == "" {
		c.JSON(200, gin.H{"code": 400, "message": "id is required"})
		return
	}

	// 将 id 转换为 uint
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		helper.GetLogger().Error("invalid id: " + idStr)
		c.JSON(200, gin.H{"code": 400, "message": "invalid id"})
		return
	}

	// 通过 id 查找服务商账号
	accountDAO := dao.NewProviderAccountDAO()
	account, err := accountDAO.GetByID(uint(id))
	if err != nil {
		helper.GetLogger().Error("account not found: " + idStr)
		c.JSON(200, gin.H{"code": 404, "message": "account not found"})
		return
	}

	providerCode := account.ProviderCode

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
