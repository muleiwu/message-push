package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// ChannelController 通道管理控制器
type ChannelController struct {
}

// CreateChannel 创建通道
func (c ChannelController) CreateChannel(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	var req dto.CreateChannelRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.CreateChannel(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to create channel: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetChannelList 获取通道列表
func (c ChannelController) GetChannelList(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	var req dto.ChannelListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.GetChannelList(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get channel list: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetChannel 获取通道详情
func (c ChannelController) GetChannel(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	resp, err := adminService.GetChannelByID(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 404, "channel not found")
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// UpdateChannel 更新通道
func (c ChannelController) UpdateChannel(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.UpdateChannelRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	if err := adminService.UpdateChannel(uint(id), &req); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to update channel: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, gin.H{"message": "updated successfully"})
}

// DeleteChannel 删除通道
func (c ChannelController) DeleteChannel(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	if err := adminService.DeleteChannel(uint(id)); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to delete channel: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, gin.H{"message": "deleted successfully"})
}

// GetChannelBindings 获取通道的模板绑定配置列表
func (c ChannelController) GetChannelBindings(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	resp, err := adminService.GetChannelBindings(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get channel bindings: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetChannelBinding 获取单个通道绑定配置
func (c ChannelController) GetChannelBinding(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	bindingIDStr := ctx.Param("bindingId")
	bindingID, err := strconv.ParseUint(bindingIDStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid binding id")
		return
	}

	resp, err := adminService.GetChannelBinding(uint(bindingID))
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get channel binding: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// UpdateChannelBinding 更新通道绑定配置
func (c ChannelController) UpdateChannelBinding(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	bindingIDStr := ctx.Param("bindingId")
	bindingID, err := strconv.ParseUint(bindingIDStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid binding id")
		return
	}

	var req dto.UpdateChannelBindingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	if err := adminService.UpdateChannelBinding(uint(bindingID), &req); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to update channel binding: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, gin.H{"message": "updated successfully"})
}

// DeleteChannelBinding 删除通道绑定配置
func (c ChannelController) DeleteChannelBinding(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	bindingIDStr := ctx.Param("bindingId")
	bindingID, err := strconv.ParseUint(bindingIDStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid binding id")
		return
	}

	if err := adminService.DeleteChannelBinding(uint(bindingID)); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to delete channel binding: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, gin.H{"message": "deleted successfully"})
}

// GetActiveChannels 获取活跃通道列表
func (c ChannelController) GetActiveChannels(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	resp, err := adminService.GetActiveChannels()
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get active channels: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// CreateChannelBinding 创建通道绑定配置
func (c ChannelController) CreateChannelBinding(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.CreateChannelBindingRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminService.CreateChannelBinding(uint(id), &req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to create channel binding: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetAvailableTemplateBindings 获取通道可用的模板绑定列表
func (c ChannelController) GetAvailableTemplateBindings(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	resp, err := adminService.GetAvailableTemplateBindings(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get available template bindings: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}
