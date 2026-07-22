// Package server 提供 identity 模块的 go-web ServerInterface 实现。
package server

import (
	"fmt"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	appHelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// BootstrapAdminServer 在无人值守部署（跳过 Web 安装向导）时引导初始管理员。
//
// 适用场景：k8s 等环境通过 APP_INSTALLED=true 跳过安装向导后，数据库虽已建表
// 但没有任何管理员账号。本服务在迁移之后运行，按需从环境变量幂等创建初始管理员。
type BootstrapAdminServer struct{}

type stringEnv interface {
	GetString(key string, defaultValue string) string
}

type bootstrapAdminSettings struct {
	username string
	password string
	realName string
	email    string
}

// NewBootstrapAdminServer 创建 BootstrapAdminServer。
func NewBootstrapAdminServer() *BootstrapAdminServer {
	return &BootstrapAdminServer{}
}

// Run 按需引导初始管理员（幂等）。
//
// 仅当满足全部条件时创建：
//  1. app.installed 为 true（已跳过/完成安装）；
//  2. 环境变量 ADMIN_USERNAME 与 ADMIN_PASSWORD 均已设置；
//  3. admin_users 表中当前没有任何管理员。
//
// 其余情况（未安装、未配置凭据、已存在管理员）均跳过，不报错。
func (receiver *BootstrapAdminServer) Run() error {
	logger := helper.GetLogger()

	if !helper.GetConfig().GetBool("app.installed", false) {
		logger.Info("系统未安装，跳过初始管理员引导")
		return nil
	}

	env := helper.GetEnv()
	settings := readBootstrapAdminSettings(env)
	if settings.username == "" || settings.password == "" {
		logger.Info("未配置 ADMIN_USERNAME/ADMIN_PASSWORD，跳过初始管理员引导")
		return nil
	}

	db := helper.GetDatabase()
	if db == nil {
		return fmt.Errorf("[bootstrap admin] 数据库连接未初始化")
	}

	created, err := createBootstrapAdmin(
		db,
		settings.username,
		settings.password,
		settings.realName,
		settings.email,
	)
	if err != nil {
		return err
	}
	if created {
		logger.Info(fmt.Sprintf("[bootstrap admin] 已创建初始管理员: %s", settings.username))
	}
	return nil
}

func readBootstrapAdminSettings(env stringEnv) bootstrapAdminSettings {
	username := env.GetString("admin.username", "")
	return bootstrapAdminSettings{
		username: username,
		password: env.GetString("admin.password", ""),
		realName: env.GetString("admin.real_name", username),
		email:    env.GetString("admin.email", ""),
	}
}

// createBootstrapAdmin 创建无人值守初始管理员。返回 false 表示已有管理员，已幂等跳过。
func createBootstrapAdmin(db *gorm.DB, username, password, realName, email string) (bool, error) {
	var count int64
	if err := db.Model(&model.AdminUser{}).Count(&count).Error; err != nil {
		return false, fmt.Errorf("[bootstrap admin] 统计管理员失败: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	normalizedEmail, err := appHelper.NormalizeAdminEmail(email)
	if err != nil {
		return false, fmt.Errorf("[bootstrap admin] 管理员邮箱无效: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, fmt.Errorf("[bootstrap admin] 密码加密失败: %w", err)
	}

	adminUser := &model.AdminUser{
		Username:   username,
		Password:   string(hashedPassword),
		RealName:   realName,
		AuthSource: "local",
		Status:     1,
	}
	if normalizedEmail != "" {
		adminUser.Email = &normalizedEmail
	}
	if err := db.Create(adminUser).Error; err != nil {
		return false, fmt.Errorf("[bootstrap admin] 创建初始管理员失败: %w", err)
	}

	return true, nil
}

// Stop 引导服务无需停止操作。
func (receiver *BootstrapAdminServer) Stop() error {
	return nil
}
