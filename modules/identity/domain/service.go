// Package domain 是 identity 限界上下文的领域层：
// 定义应用身份管理（ApplicationService）、管理员用户管理（UserService）
// 与系统安装（InstallService）三个服务端口。
package domain

import (
	"errors"

	"cnb.cool/mliev/push/message-push/app/dto"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	// ErrAdminUserNotFound 表示管理员不存在或已被删除。
	ErrAdminUserNotFound = errors.New("管理员账号不存在")
	// ErrAdminUsernameConflict 表示用户名已被占用。
	ErrAdminUsernameConflict = errors.New("用户名已存在")
	// ErrAdminEmailConflict 表示邮箱已被其他管理员占用。
	ErrAdminEmailConflict = errors.New("邮箱已被使用")
	// ErrAdminEmailImmutable 为兼容旧调用方保留；管理员资料更新已不再返回此错误。
	// Deprecated: SSO 管理员邮箱现在允许在后台维护。
	ErrAdminEmailImmutable = errors.New("SSO 账号邮箱由身份提供方管理")
	// ErrAdminPasswordResetForbidden 表示 SSO 自动创建的账号不支持本地密码重置。
	ErrAdminPasswordResetForbidden = errors.New("SSO 账号不支持重置本地密码")
	// ErrInvalidAdminEmail 表示邮箱缺失或格式不合法。
	ErrInvalidAdminEmail = errors.New("邮箱格式不正确")
	// ErrInvalidAdminStatus 表示账号状态不在兼容范围内。
	ErrInvalidAdminStatus = errors.New("管理员账号状态不正确")
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
