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

	// 解析表单数据（支持 application/x-www-form-urlencoded 和 multipart/form-data）
	_ = c.Request.ParseMultipartForm(32 << 20) // 32MB
	formData := make(map[string]string)
	if c.Request.PostForm != nil {
		for key, values := range c.Request.PostForm {
			if len(values) > 0 {
				formData[key] = values[0]
			}
		}
	}

	// 构造回调请求
	req := &sender.CallbackRequest{
		ProviderCode: providerCode,
		RawBody:      rawBody,
		Headers:      make(map[string]string),
		QueryParams:  make(map[string]string),
		FormData:     formData,
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
	resp := callbackService.HandleCallback(c.Request.Context(), providerCode, req)

	// 如果状态码为 0，自动设置为 500
	statusCode := resp.StatusCode
	if statusCode == 0 {
		statusCode = 500
	}

	// 返回供应商期望的响应格式
	c.String(statusCode, resp.Body)
}
