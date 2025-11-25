package controller

import (
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
	"github.com/gin-gonic/gin"
)

// MessageController 消息控制器
type MessageController struct {
}

// Send 发送消息
func (ctrl MessageController) Send(c *gin.Context, helper interfaces.HelperInterface) {
	messageService := service.NewMessageService()
	var req service.SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithMessage(c, "invalid request: "+err.Error())
		return
	}

	// 从上下文获取认证信息（已由中间件验证）
	appID, _ := c.Get("app_id")
	req.AppID = appID.(string)

	resp, err := messageService.Send(c.Request.Context(), &req)
	if err != nil {
		FailWithMessage(c, err.Error())
		return
	}

	SuccessWithData(c, resp)
}

// BatchSend 批量发送消息
func (ctrl MessageController) BatchSend(c *gin.Context, helper interfaces.HelperInterface) {
	messageService := service.NewMessageService()
	var req service.BatchSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithMessage(c, "invalid request: "+err.Error())
		return
	}

	// 从上下文获取认证信息（已由中间件验证）
	appID, _ := c.Get("app_id")
	req.AppID = appID.(string)

	resp, err := messageService.BatchSend(c.Request.Context(), &req)
	if err != nil {
		FailWithMessage(c, err.Error())
		return
	}

	SuccessWithData(c, resp)
}

// QueryTask 查询任务状态
func (ctrl MessageController) QueryTask(c *gin.Context, helper interfaces.HelperInterface) {
	messageService := service.NewMessageService()
	taskID := c.Param("task_id")
	if taskID == "" {
		FailWithMessage(c, "task_id is required")
		return
	}

	task, err := messageService.QueryTask(c.Request.Context(), taskID)
	if err != nil {
		FailWithMessage(c, err.Error())
		return
	}

	SuccessWithData(c, task)
}
