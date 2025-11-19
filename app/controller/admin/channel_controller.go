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

// BindProviderToChannel 绑定服务商到通道
func (c ChannelController) BindProviderToChannel(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminService := service.NewAdminChannelService()
	var req dto.BindProviderToChannelRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	if err := adminService.BindProviderToChannel(&req); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to bind provider to channel: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, gin.H{"message": "bound successfully"})
}
