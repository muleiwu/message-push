package admin

import (
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
)

// CallbackController 服务商回调记录管理控制器（统一下行回执与上行短信）
type CallbackController struct {
}

// GetCallbackList 获取回调记录列表
func (c CallbackController) GetCallbackList(ctx httpInterfaces.RouterContextInterface) {
	adminService := service.NewAdminCallbackService()
	var req dto.CallbackListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.GetCallbackList(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get callbacks: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}
