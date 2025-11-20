package service

import (
	"context"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// InstallService 安装服务
type InstallService struct {
	db *gorm.DB
}

// NewInstallService 创建安装服务实例
func NewInstallService(db *gorm.DB) *InstallService {
	return &InstallService{
		db: db,
	}
}

// CheckInstallStatus 检查系统安装状态
func (s *InstallService) CheckInstallStatus() dto.InstallCheckResponse {
	response := dto.InstallCheckResponse{
		Installed:          false,
		DatabaseConnected:  false,
		DatabaseConfigured: false,
	}

	// 检查数据库是否配置
	if s.db != nil {
		response.DatabaseConfigured = true

		// 检查数据库连接
		sqlDB, err := s.db.DB()
		if err == nil {
			if err := sqlDB.Ping(); err == nil {
				response.DatabaseConnected = true

				// 获取当前数据库类型
				response.CurrentDatabase = s.db.Dialector.Name()
			}
		}
	}

	// 检查是否已有管理员用户（判断是否已安装）
	if response.DatabaseConnected {
		var count int64
		if err := s.db.Model(&model.AdminUser{}).Count(&count).Error; err == nil && count > 0 {
			response.Installed = true
			response.Message = "系统已安装"
		} else {
			response.Message = "系统未安装"
		}
	} else {
		response.Message = "数据库未连接"
	}

	return response
}

// TestDatabaseConnection 测试数据库连接
func (s *InstallService) TestDatabaseConnection(config dto.DatabaseConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector
	var dsn string

	driver := config.Driver
	if driver == "" {
		driver = "mysql" // 默认使用 MySQL
	}

	switch driver {
	case "mysql":
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			config.Username,
			config.Password,
			config.Host,
			config.Port,
			config.Database,
			getCharset(config.Charset))
		dialector = mysql.Open(dsn)

	case "postgresql", "postgres":
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
			config.Host,
			config.Port,
			config.Username,
			config.Password,
			config.Database)
		dialector = postgres.Open(dsn)

	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s (仅支持 mysql 和 postgresql)", driver)
	}

	// 尝试连接数据库
	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}

	// 测试连接
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接失败: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库 Ping 失败: %w", err)
	}

	return db, nil
}

// TestRedisConnection 测试 Redis 连接
func (s *InstallService) TestRedisConnection(config dto.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis 连接失败: %w", err)
	}

	return client, nil
}

// UpdateDatabaseConfig 更新数据库配置到配置文件
func (s *InstallService) UpdateDatabaseConfig(config dto.DatabaseConfig) error {
	// 读取配置文件
	driver := config.Driver
	if driver == "" {
		driver = "mysql"
	}

	// 更新数据库配置
	viper.Set("database.driver", driver)
	viper.Set("database.host", config.Host)
	viper.Set("database.port", config.Port)
	viper.Set("database.username", config.Username)
	viper.Set("database.password", config.Password)
	viper.Set("database.dbname", config.Database)

	if config.Charset != "" {
		viper.Set("database.charset", config.Charset)
	}

	// 写入配置文件
	// 先尝试 SafeWriteConfig（如果文件不存在会创建）
	viper.SetConfigFile("./config/config.yaml")
	if err := viper.SafeWriteConfig(); err != nil {
		// 如果文件已存在，使用 WriteConfig 更新
		if err := viper.WriteConfig(); err != nil {
			return fmt.Errorf("写入数据库配置失败: %w", err)
		}
	}
	return nil
}

// UpdateRedisConfig 更新 Redis 配置到配置文件
func (s *InstallService) UpdateRedisConfig(config dto.RedisConfig) error {

	// 更新 Redis 配置
	viper.Set("redis.host", config.Host)
	viper.Set("redis.port", config.Port)
	viper.Set("redis.password", config.Password)
	viper.Set("redis.db", config.DB)

	// 写入配置文件
	// 先尝试 SafeWriteConfig（如果文件不存在会创建）
	viper.SetConfigFile("./config/config.yaml")
	if err := viper.SafeWriteConfig(); err != nil {
		// 如果文件已存在，使用 WriteConfig 更新
		if err := viper.WriteConfig(); err != nil {
			return fmt.Errorf("写入数据库配置失败: %w", err)
		}
	}
	return nil
}

// CreateInitialData 创建初始数据（管理员账户）
func (s *InstallService) CreateInitialData(admin dto.AdminAccountInfo) error {
	// 验证必填字段
	if admin.Username == "" || admin.Password == "" || admin.Email == "" || admin.RealName == "" {
		return fmt.Errorf("管理员信息不完整")
	}

	// 检查用户名是否已存在（使用 service 内部的数据库连接）
	var count int64
	if err := s.db.Model(&model.AdminUser{}).Where("username = ?", admin.Username).Count(&count).Error; err != nil {
		return fmt.Errorf("检查用户名失败: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("用户名 %s 已存在", admin.Username)
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(admin.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	// 创建管理员用户（使用 service 内部的数据库连接）
	adminUser := &model.AdminUser{
		Username: admin.Username,
		Password: string(hashedPassword),
		RealName: admin.RealName,
		Status:   1, // 启用状态
	}

	if err := s.db.Create(adminUser).Error; err != nil {
		return fmt.Errorf("创建管理员账户失败: %w", err)
	}

	return nil
}

// MarkAsInstalled 标记系统为已安装
func (s *InstallService) MarkAsInstalled() error {
	// 设置安装标记
	viper.Set("app.installed", true)

	// 写入配置文件
	// 先尝试 SafeWriteConfig（如果文件不存在会创建）
	viper.SetConfigFile("./config.yaml")
	if err := viper.SafeWriteConfig(); err != nil {
		// 如果文件已存在，使用 WriteConfig 更新
		if err := viper.WriteConfig(); err != nil {
			return fmt.Errorf("标记系统已安装失败: %w", err)
		}
	}

	return nil
}

// getCharset 获取字符集，如果为空则返回默认值
func getCharset(charset string) string {
	if charset == "" {
		return "utf8mb4"
	}
	return charset
}
