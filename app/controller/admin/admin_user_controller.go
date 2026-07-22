package admin

import (
	"errors"
	"strconv"

	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/modules/identity"
)

// AdminUserController 管理员用户管理控制器
type AdminUserController struct {
}

// GetUserList 获取管理员用户列表
func (c AdminUserController) GetUserList(ctx httpInterfaces.RouterContextInterface) {
	adminUserService := identity.GetUserService()
	var req dto.AdminUserListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminUserService.GetUserList(&req)
	if err != nil {
		writeAdminUserServiceError(ctx, err, "failed to get user list")
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// CreateUser 创建管理员用户
func (c AdminUserController) CreateUser(ctx httpInterfaces.RouterContextInterface) {
	adminUserService := identity.GetUserService()
	var req dto.CreateAdminUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	resp, err := adminUserService.CreateUser(&req)
	if err != nil {
		writeAdminUserServiceError(ctx, err, "failed to create user")
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// GetUser 获取管理员用户详情
func (c AdminUserController) GetUser(ctx httpInterfaces.RouterContextInterface) {
	adminUserService := identity.GetUserService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	resp, err := adminUserService.GetUserByID(uint(id))
	if err != nil {
		writeAdminUserServiceError(ctx, err, "failed to get user")
		return
	}

	controller.SuccessResponse(ctx, resp)
}

// UpdateUser 更新管理员用户
func (c AdminUserController) UpdateUser(ctx httpInterfaces.RouterContextInterface) {
	adminUserService := identity.GetUserService()
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
		writeAdminUserServiceError(ctx, err, "failed to update user")
		return
	}

	controller.SuccessResponse(ctx, map[string]any{"message": "updated successfully"})
}

// DeleteUser 删除管理员用户
func (c AdminUserController) DeleteUser(ctx httpInterfaces.RouterContextInterface) {
	adminUserService := identity.GetUserService()
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	if err := adminUserService.DeleteUser(uint(id)); err != nil {
		writeAdminUserServiceError(ctx, err, "failed to delete user")
		return
	}

	controller.SuccessResponse(ctx, map[string]any{"message": "deleted successfully"})
}

// ResetPassword 重置管理员用户密码
func (c AdminUserController) ResetPassword(ctx httpInterfaces.RouterContextInterface) {
	adminUserService := identity.GetUserService()
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
		writeAdminUserServiceError(ctx, err, "failed to reset password")
		return
	}

	controller.SuccessResponse(ctx, resp)
}

func writeAdminUserServiceError(ctx httpInterfaces.RouterContextInterface, err error, fallback string) {
	status := adminUserServiceErrorStatus(err)
	if status == 500 {
		controller.ErrorResponse(ctx, 500, fallback+": "+err.Error())
		return
	}
	controller.ErrorResponse(ctx, status, err.Error())
}

func adminUserServiceErrorStatus(err error) int {
	switch {
	case errors.Is(err, identity.ErrInvalidAdminEmail), errors.Is(err, identity.ErrInvalidAdminStatus):
		return 400
	case errors.Is(err, identity.ErrAdminUserNotFound):
		return 404
	case errors.Is(err, identity.ErrAdminEmailImmutable),
		errors.Is(err, identity.ErrAdminPasswordResetForbidden):
		return 403
	case errors.Is(err, identity.ErrAdminUsernameConflict),
		errors.Is(err, identity.ErrAdminEmailConflict):
		return 409
	default:
		return 500
	}
}
