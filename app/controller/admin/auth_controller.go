package admin

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// AuthController 认证控制器
type AuthController struct {
}

// Login 管理员登录
func (c AuthController) Login(ctx *gin.Context, h interfaces.HelperInterface) {
	var req dto.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "用户名和密码不能为空")
		return
	}

	// 查询用户
	userDAO := dao.NewAdminUserDAO()
	user, err := userDAO.GetByUsername(req.Username)
	if err != nil {
		controller.ErrorResponse(ctx, 401, "用户名或密码错误")
		return
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		controller.ErrorResponse(ctx, 401, "用户名或密码错误")
		return
	}

	// 生成 token
	token, err := helper.GenerateToken(user.ID, user.Username)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "生成令牌失败")
		return
	}

	// 返回响应
	resp := dto.LoginResponse{
		AccessToken: token,
	}
	controller.SuccessResponse(ctx, resp)
}

// Logout 管理员登出
func (c AuthController) Logout(ctx *gin.Context, h interfaces.HelperInterface) {
	controller.SuccessResponse(ctx, gin.H{"message": "退出成功"})
}

// GetUserInfo 获取用户信息
func (c AuthController) GetUserInfo(ctx *gin.Context, h interfaces.HelperInterface) {
	// 从上下文中获取用户信息
	userID, exists := ctx.Get("user_id")
	if !exists {
		controller.ErrorResponse(ctx, 401, "未授权")
		return
	}

	username, _ := ctx.Get("username")

	// 查询用户详细信息
	userDAO := dao.NewAdminUserDAO()
	user, err := userDAO.GetByID(userID.(uint))
	if err != nil {
		controller.ErrorResponse(ctx, 404, "用户不存在")
		return
	}

	// 返回用户信息
	resp := dto.UserInfoResponse{
		UserID:   user.ID,
		Username: user.Username,
		RealName: user.RealName,
		Roles:    []string{"admin"},
		HomePath: "/dashboard",
	}

	// 避免未使用的变量警告
	_ = username

	controller.SuccessResponse(ctx, resp)
}

// GetAccessCodes 获取权限码
func (c AuthController) GetAccessCodes(ctx *gin.Context, h interfaces.HelperInterface) {
	// 返回管理员权限码
	codes := dto.AccessCodesResponse{
		"AC_100000", // 超级管理员权限
		"AC_100100", // 应用管理权限
		"AC_100200", // 服务商管理权限
		"AC_100300", // 通道管理权限
		"AC_100400", // 统计查询权限
	}
	controller.SuccessResponse(ctx, codes)
}
