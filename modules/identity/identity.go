// Package identity 是 identity 限界上下文的对外门面（facade）。
//
// ApplicationService / UserService 经 DI 容器解析；InstallService 因需以待校验的
// 数据库连接构造，故由 NewInstallService(db) 直接创建（非容器单例）。
package identity

import (
	"cnb.cool/mliev/open/go-web/pkg/container"
	"cnb.cool/mliev/push/message-push/modules/identity/domain"
	"cnb.cool/mliev/push/message-push/modules/identity/infrastructure"
	"gorm.io/gorm"
)

type (
	ApplicationService = domain.ApplicationService
	UserService        = domain.UserService
	InstallService     = domain.InstallService
)

// GetApplicationService 从 DI 容器解析应用管理服务。
func GetApplicationService() domain.ApplicationService {
	return container.MustGet[domain.ApplicationService]()
}

// GetUserService 从 DI 容器解析管理员用户服务。
func GetUserService() domain.UserService {
	return container.MustGet[domain.UserService]()
}

// NewInstallService 以指定数据库连接创建安装服务（安装流程使用待校验的连接）。
func NewInstallService(db *gorm.DB) domain.InstallService {
	return infrastructure.NewInstallService(db)
}
