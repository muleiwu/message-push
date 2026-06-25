package controller

import (
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"

	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/modules/messaging"
)

// MessageController 消息控制器
type MessageController struct {
}

// Send 发送消息
func (ctrl MessageController) Send(c httpInterfaces.RouterContextInterface) {
	messageService := messaging.GetService()
	var req dto.SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithMessage(c, "invalid request: "+err.Error())
		return
	}

	// 从上下文获取认证信息（已由中间件验证）
	appID := c.Get("app_id")
	req.AppID = appID.(string)

	resp, err := messageService.Send(c.Request().Context(), &req)
	if err != nil {
		FailWithMessage(c, err.Error())
		return
	}

	SuccessWithData(c, resp)
}

// BatchSend 批量发送消息
func (ctrl MessageController) BatchSend(c httpInterfaces.RouterContextInterface) {
	messageService := messaging.GetService()
	var req dto.BatchSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		FailWithMessage(c, "invalid request: "+err.Error())
		return
	}

	// 从上下文获取认证信息（已由中间件验证）
	appID := c.Get("app_id")
	req.AppID = appID.(string)

	resp, err := messageService.BatchSend(c.Request().Context(), &req)
	if err != nil {
		FailWithMessage(c, err.Error())
		return
	}

	SuccessWithData(c, resp)
}

// QueryTask 查询任务状态
func (ctrl MessageController) QueryTask(c httpInterfaces.RouterContextInterface) {
	messageService := messaging.GetService()
	taskID := c.Param("task_id")
	if taskID == "" {
		FailWithMessage(c, "task_id is required")
		return
	}

	task, err := messageService.QueryTask(c.Request().Context(), taskID)
	if err != nil {
		FailWithMessage(c, err.Error())
		return
	}

	SuccessWithData(c, task)
}
