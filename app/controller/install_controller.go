package controller

import (
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
	"cnb.cool/mliev/push/message-push/internal/pkg/reload"
	"github.com/gin-gonic/gin"
)

// InstallController 系统安装控制器
type InstallController struct {
	BaseResponse
}

// CheckInstall 检查系统安装状态
// GET /api/install/check
func (ic InstallController) CheckInstall(c *gin.Context, helper interfaces.HelperInterface) {
	installService := service.NewInstallService(helper.GetDatabase())
	response := installService.CheckInstallStatus()

	ic.Success(c, response)
}

// TestConnection 测试数据库连接
// POST /api/install/test-connection
func (ic InstallController) TestConnection(c *gin.Context, helper interfaces.HelperInterface) {
	var req dto.DatabaseConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		ic.Error(c, constants.CodeBadRequest, "请求参数错误: "+err.Error())
		return
	}

	installService := service.NewInstallService(helper.GetDatabase())

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
func (ic InstallController) TestRedisConnection(c *gin.Context, helper interfaces.HelperInterface) {
	var req dto.RedisConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		ic.Error(c, constants.CodeBadRequest, "请求参数错误: "+err.Error())
		return
	}

	installService := service.NewInstallService(helper.GetDatabase())

	// 测试 Redis 连接
	testRedis, err := installService.TestRedisConnection(req)
	if err != nil {
		ic.Error(c, constants.CodeBadRequest, "Redis 连接测试失败: "+err.Error())
		return
	}

	// 测试成功后关闭连接
	if err := testRedis.Close(); err != nil {
		helper.GetLogger().Warn("关闭 Redis 测试连接时出错: " + err.Error())
	}

	ic.SuccessWithMessage(c, "Redis 连接测试成功", nil)
}

// SubmitInstall 提交系统安装
// POST /api/install/submit
func (ic InstallController) SubmitInstall(c *gin.Context, helper interfaces.HelperInterface) {
	var req dto.InstallSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ic.Error(c, constants.CodeBadRequest, "请求参数错误: "+err.Error())
		return
	}

	installService := service.NewInstallService(helper.GetDatabase())

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
			helper.GetLogger().Warn("关闭 Redis 测试连接时出错: " + err.Error())
		}
	}()

	// 4. 使用测试成功的数据库连接创建新的 service 实例
	newInstallService := service.NewInstallService(testDB)

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

	// 7. 执行数据库迁移（自动创建表）
	if err := testDB.AutoMigrate(
		&model.AdminUser{},
		&model.Application{},
		&model.Provider{},
		&model.ProviderChannel{},
		&model.Channel{},
		&model.ChannelProviderRelation{},
		&model.PushTask{},
		&model.PushBatchTask{},
		&model.PushLog{},
	); err != nil {
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
