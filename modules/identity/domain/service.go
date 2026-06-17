// Package domain 是 identity 限界上下文的领域层：
// 定义应用身份管理（ApplicationService）、管理员用户管理（UserService）
// 与系统安装（InstallService）三个服务端口。
package domain

import (
	"cnb.cool/mliev/push/message-push/app/dto"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ApplicationService 应用（接入方）管理端口：应用的增删改查、密钥重置与配额查询。
type ApplicationService interface {
	CreateApplication(req *dto.CreateApplicationRequest) (*dto.ApplicationResponse, error)
	GetApplicationList(req *dto.ApplicationListRequest) (*dto.ApplicationListResponse, error)
	GetApplicationByID(id uint) (*dto.ApplicationResponse, error)
	UpdateApplication(id uint, req *dto.UpdateApplicationRequest) error
	DeleteApplication(id uint) error
	RegenerateSecret(appID uint) (*dto.RegenerateSecretResponse, error)
	GetQuotaUsage(id uint) (*dto.QuotaUsageResponse, error)
}

// UserService 管理员用户管理端口。
type UserService interface {
	CreateUser(req *dto.CreateAdminUserRequest) (*dto.AdminUserResponse, error)
	GetUserList(req *dto.AdminUserListRequest) (*dto.AdminUserListResponse, error)
	GetUserByID(id uint) (*dto.AdminUserResponse, error)
	UpdateUser(id uint, req *dto.UpdateAdminUserRequest) error
	DeleteUser(id uint) error
	ResetPassword(id uint, req *dto.ResetPasswordRequest) (*dto.ResetPasswordResponse, error)
}

// InstallService 系统安装端口。安装时以待校验的数据库连接构造（非容器单例），
// 由 facade 的 NewInstallService(db) 创建。
type InstallService interface {
	CheckInstallStatus() dto.InstallCheckResponse
	TestDatabaseConnection(config dto.DatabaseConfig) (*gorm.DB, error)
	TestRedisConnection(config dto.RedisConfig) (*redis.Client, error)
	UpdateDatabaseConfig(config dto.DatabaseConfig) error
	UpdateRedisConfig(config dto.RedisConfig) error
	CreateInitialData(admin dto.AdminAccountInfo) error
	MarkAsInstalled() error
}
