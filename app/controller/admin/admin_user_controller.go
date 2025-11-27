package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// AdminUserController 管理员用户管理控制器
type AdminUserController struct {
}

// GetUserList 获取管理员用户列表
func (c AdminUserController) GetUserList(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminUserService := service.NewAdminUserService()
	var req dto.AdminUserListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminUserService.GetUserList(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get user list: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// CreateUser 创建管理员用户
func (c AdminUserController) CreateUser(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminUserService := service.NewAdminUserService()
	var req dto.CreateAdminUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminUserService.CreateUser(&req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to create user: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetUser 获取管理员用户详情
func (c AdminUserController) GetUser(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminUserService := service.NewAdminUserService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	resp, err := adminUserService.GetUserByID(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 404, "user not found")
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// UpdateUser 更新管理员用户
func (c AdminUserController) UpdateUser(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminUserService := service.NewAdminUserService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.UpdateAdminUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	if err := adminUserService.UpdateUser(uint(id), &req); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to update user: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, gin.H{"message": "updated successfully"})
}

// DeleteUser 删除管理员用户
func (c AdminUserController) DeleteUser(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminUserService := service.NewAdminUserService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	if err := adminUserService.DeleteUser(uint(id)); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to delete user: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, gin.H{"message": "deleted successfully"})
}

// ResetPassword 重置管理员用户密码
func (c AdminUserController) ResetPassword(ctx *gin.Context, helper interfaces.HelperInterface) {
	adminUserService := service.NewAdminUserService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.ResetPasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminUserService.ResetPassword(uint(id), &req)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to reset password: "+err.Error())
		return
	}

	controller.SuccessResponse(ctx, resp)
}
