package controller

import (
	"fmt"
	"net/http"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/open/go-web/pkg/server/reload"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	appHelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/migration"
	"cnb.cool/mliev/push/message-push/modules/identity"
)

// InstallController 系统安装控制器
type InstallController struct {
	BaseResponse
}

// CheckInstall 检查系统安装状态
// GET /api/install/check
func (ic InstallController) CheckInstall(c httpInterfaces.RouterContextInterface) {
	installService := identity.NewInstallService(helper.GetDatabase())
	response := installService.CheckInstallStatus()

	ic.Success(c, response)
}

// TestConnection 测试数据库连接
// POST /api/install/test-connection
func (ic InstallController) TestConnection(c httpInterfaces.RouterContextInterface) {
	var req dto.DatabaseConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		ic.Error(c, constants.CodeBadRequest, "请求参数错误: "+err.Error())
		return
	}

	installService := identity.NewInstallService(helper.GetDatabase())

	// 测试数据库连接
	testDB, err := installService.TestDatabaseConnection(req)
	if err != nil {
		ic.Error(c, constants.CodeBadRequest, "数据库连接测试失败: "+err.Error())
		return
	}

	// 测试成功后关闭连接
	if sqlDB, err := testDB.DB(); err == nil {
		sqlDB.Close()
	}

	ic.SuccessWithMessage(c, "数据库连接测试成功", nil)
}

// TestRedisConnection 测试 Redis 连接
// POST /api/install/test-redis
func (ic InstallController) TestRedisConnection(c httpInterfaces.RouterContextInterface) {
	var req dto.RedisConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		ic.Error(c, constants.CodeBadRequest, "请求参数错误: "+err.Error())
		return
	}

	installService := identity.NewInstallService(helper.GetDatabase())

	// 测试 Redis 连接
	testRedis, err := installService.TestRedisConnection(req)
	if err != nil {
		ic.Error(c, constants.CodeBadRequest, "Redis 连接测试失败: "+err.Error())
		return
	}

	// 测试成功后关闭连接
	if err := testRedis.Close(); err != nil {
		helper.GetRequestLogger(c).Warn("关闭 Redis 测试连接时出错: " + err.Error())
	}

	ic.SuccessWithMessage(c, "Redis 连接测试成功", nil)
}

// SubmitInstall 提交系统安装
// POST /api/install/submit
func (ic InstallController) SubmitInstall(c httpInterfaces.RouterContextInterface) {
	var req dto.InstallSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ic.Error(c, constants.CodeBadRequest, "请求参数错误: "+err.Error())
		return
	}
	normalizedAdmin, err := normalizeInstallAdmin(req.Admin)
	if err != nil {
		ic.Error(c, http.StatusBadRequest, "管理员信息错误: "+err.Error())
		return
	}
	req.Admin = normalizedAdmin

	installService := identity.NewInstallService(helper.GetDatabase())

	// 1. 检查系统是否已安装
	status := installService.CheckInstallStatus()
	if status.Installed {
		ic.Error(c, constants.CodeBadRequest, "系统已安装，无法重复安装")
		return
	}

	// 2. 测试数据库连接并获取连接实例
	testDB, err := installService.TestDatabaseConnection(req.Database)
	if err != nil {
		ic.Error(c, constants.CodeBadRequest, "数据库连接测试失败: "+err.Error())
		return
	}

	// 确保在函数退出时关闭数据库连接
	defer func() {
		if sqlDB, err := testDB.DB(); err == nil {
			sqlDB.Close()
		}
	}()

	// 3. 测试 Redis 连接
	testRedis, err := installService.TestRedisConnection(req.Redis)
	if err != nil {
		ic.Error(c, constants.CodeBadRequest, "Redis 连接测试失败: "+err.Error())
		return
	}

	// 确保在函数退出时关闭 Redis 连接
	defer func() {
		if err := testRedis.Close(); err != nil {
			helper.GetRequestLogger(c).Warn("关闭 Redis 测试连接时出错: " + err.Error())
		}
	}()

	// 4. 使用测试成功的数据库连接创建新的 service 实例
	newInstallService := identity.NewInstallService(testDB)

	// 5. 更新数据库配置到配置文件
	if err := newInstallService.UpdateDatabaseConfig(req.Database); err != nil {
		ic.Error(c, constants.CodeInternalError, "更新数据库配置失败: "+err.Error())
		return
	}

	// 6. 更新 Redis 配置到配置文件
	if err := newInstallService.UpdateRedisConfig(req.Redis); err != nil {
		ic.Error(c, constants.CodeInternalError, "更新 Redis 配置失败: "+err.Error())
		return
	}

	// 7. 执行数据库迁移（使用 goose 版本化迁移）
	sqlDB, err := testDB.DB()
	if err != nil {
		ic.Error(c, constants.CodeInternalError, "获取数据库连接失败: "+err.Error())
		return
	}

	// 执行 goose 迁移，使用用户选择的数据库驱动作为方言
	dialect := req.Database.Driver
	if dialect == "" {
		dialect = "mysql" // 默认 MySQL
	}
	if err := migration.RunGooseMigrations(sqlDB, dialect, nil); err != nil {
		ic.Error(c, constants.CodeInternalError, "数据库迁移失败: "+err.Error())
		return
	}

	// 8. 创建初始数据（管理员账户）
	if err := newInstallService.CreateInitialData(req.Admin); err != nil {
		ic.Error(c, constants.CodeInternalError, "创建初始数据失败: "+err.Error())
		return
	}

	// 9. 标记系统为已安装
	if err := newInstallService.MarkAsInstalled(); err != nil {
		ic.Error(c, constants.CodeInternalError, "标记系统已安装失败: "+err.Error())
		return
	}

	// 10. 返回成功响应
	response := dto.InstallSubmitResponse{
		Success: true,
		Message: "系统安装成功，服务将自动重启以应用新配置，请稍后使用管理员账户登录",
	}

	ic.SuccessWithMessage(c, "系统安装成功", response)

	// 11. 异步触发配置重载（在响应返回后）
	go func() {
		time.Sleep(500 * time.Millisecond) // 短暂延迟，确保响应已发送
		reload.TriggerReload()
		helper.GetLogger().Info("安装完成，已触发配置重载")
	}()
}

// normalizeInstallAdmin 在连接测试、配置写入和迁移前完成管理员输入校验。
// CreateInitialData 会再次校验，防止其他调用方绕过控制器。
func normalizeInstallAdmin(admin dto.AdminAccountInfo) (dto.AdminAccountInfo, error) {
	if admin.Username == "" || admin.Password == "" || admin.RealName == "" {
		return admin, fmt.Errorf("管理员信息不完整")
	}
	normalizedEmail, err := appHelper.NormalizeAdminEmail(admin.Email)
	if err != nil {
		return admin, fmt.Errorf("管理员邮箱无效: %w", err)
	}
	if normalizedEmail == "" {
		return admin, fmt.Errorf("管理员邮箱不能为空")
	}
	admin.Email = normalizedEmail
	return admin, nil
}
