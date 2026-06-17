// Package migration 提供基于 goose 的版本化数据库迁移能力，以及 go-web ServerInterface 实现。
//
// 迁移文件为 Go 代码（migrations 包，通过 init() 注册到 goose 全局注册表），
// 其中 initial_schema 通过 migrations.SetExternalDB 注入的 GORM 连接执行 AutoMigrate。
package migration

import (
	"database/sql"
	"fmt"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	// 导入迁移文件以触发 init() 注册
	"cnb.cool/mliev/push/message-push/migrations"
	"github.com/muleiwu/gsr"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

// Logger 定义迁移过程使用的日志接口。
type Logger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

// simpleLogger 在没有 gsr.Logger 时使用的简单实现（如安装阶段）。
type simpleLogger struct{}

func (l *simpleLogger) Info(msg string)  { fmt.Println("[INFO]", msg) }
func (l *simpleLogger) Warn(msg string)  { fmt.Println("[WARN]", msg) }
func (l *simpleLogger) Error(msg string) { fmt.Println("[ERROR]", msg) }

// gsrLoggerAdapter 将 gsr.Logger 适配为本包的 Logger 接口。
type gsrLoggerAdapter struct {
	logger gsr.Logger
}

func (l *gsrLoggerAdapter) Info(msg string)  { l.logger.Info(msg) }
func (l *gsrLoggerAdapter) Warn(msg string)  { l.logger.Warn(msg) }
func (l *gsrLoggerAdapter) Error(msg string) { l.logger.Error(msg) }

// RunGooseMigrations 执行 goose 版本化迁移（简单版本，供安装控制器使用）。
// 安装控制器需要先调用 migrations.SetExternalDB() 设置 GORM 连接。
// dialect: 数据库方言，如 mysql, postgres, sqlite3。
func RunGooseMigrations(db *sql.DB, dialect string, logger Logger) error {
	return RunGooseMigrationsWithGorm(db, nil, dialect, logger)
}

// RunGooseMigrationsWithGorm 执行 goose 版本化迁移（完整版本）。
// gormDB 用于 initial_schema 迁移中的 AutoMigrate。
func RunGooseMigrationsWithGorm(db *sql.DB, gormDB *gorm.DB, dialect string, logger Logger) error {
	if logger == nil {
		logger = &simpleLogger{}
	}

	// 如果提供了 GORM DB，设置外部连接供 initial_schema 使用
	if gormDB != nil {
		migrations.SetExternalDB(gormDB)
		defer migrations.ClearExternalDB()
	}

	gooseDialect := mapDriverToGooseDialect(dialect)
	if err := goose.SetDialect(gooseDialect); err != nil {
		return fmt.Errorf("设置 goose 方言失败 (%s -> %s): %w", dialect, gooseDialect, err)
	}

	currentVersion, err := goose.GetDBVersion(db)
	if err != nil {
		logger.Info("[db migration] 初始化 goose 迁移表...")
		currentVersion = 0
	} else {
		logger.Info(fmt.Sprintf("[db migration] 当前数据库版本: %d", currentVersion))
	}

	// 使用 UpByOne 循环执行，便于逐条记录
	for {
		err := goose.UpByOne(db, ".")
		if err != nil {
			if err == goose.ErrNoNextVersion {
				break
			}
			return fmt.Errorf("执行迁移失败: %w", err)
		}
	}

	newVersion, _ := goose.GetDBVersion(db)
	if newVersion != currentVersion {
		logger.Info(fmt.Sprintf("[db migration] goose 迁移完成: 版本 %d -> %d", currentVersion, newVersion))
	} else {
		logger.Info("[db migration] goose 迁移已是最新版本")
	}

	return nil
}

// mapDriverToGooseDialect 将数据库驱动名称映射为 goose 方言。
func mapDriverToGooseDialect(driver string) string {
	switch driver {
	case "postgres", "postgresql":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	case "sqlite", "sqlite3":
		return "sqlite3"
	case "mssql", "sqlserver":
		return "mssql"
	default:
		if driver == "" {
			return "mysql"
		}
		return driver
	}
}

// Server 是 go-web ServerInterface 的迁移实现：在 app.installed 为 true 时执行迁移。
type Server struct{}

// Run 执行数据库迁移（仅在系统已安装时）。
func (receiver *Server) Run() error {
	config := helper.GetConfig()
	if !config.GetBool("app.installed", false) {
		helper.GetLogger().Warn("[db migration] 数据库未安装，不执行迁移")
		return nil
	}

	gormDB := helper.GetDatabase()
	if gormDB == nil {
		return fmt.Errorf("[db migration] 数据库连接未初始化")
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("[db migration] 获取 sql.DB 失败: %w", err)
	}

	dialect := config.GetString("database.driver", "mysql")
	logger := &gsrLoggerAdapter{helper.GetLogger()}
	if err := RunGooseMigrationsWithGorm(sqlDB, gormDB, dialect, logger); err != nil {
		return fmt.Errorf("[db migration] goose 迁移失败: %w", err)
	}
	return nil
}

// Stop 迁移服务无需停止操作。
func (receiver *Server) Stop() error {
	return nil
}
