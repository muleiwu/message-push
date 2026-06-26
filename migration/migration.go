// Package migration 提供基于 goose 的版本化数据库迁移能力，以及 go-web ServerInterface 实现。
//
// 迁移文件为按数据库方言分目录的 SQL 文件（内嵌于 migrations 包的 embed.FS），
// 运行时根据 database.driver 选择 mysql/pgsql/sqlite 子目录执行。
package migration

import (
	"database/sql"
	"fmt"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/migrations"
	"github.com/muleiwu/gsr"
	"github.com/pressly/goose/v3"
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

// RunGooseMigrations 执行 goose 版本化迁移。
// dialect: 数据库驱动名称，如 mysql, postgresql, sqlite。
func RunGooseMigrations(db *sql.DB, dialect string, logger Logger) error {
	if logger == nil {
		logger = &simpleLogger{}
	}

	gooseDialect := mapDriverToGooseDialect(dialect)
	if err := goose.SetDialect(gooseDialect); err != nil {
		return fmt.Errorf("设置 goose 方言失败 (%s -> %s): %w", dialect, gooseDialect, err)
	}

	// 使用内嵌的 SQL 迁移文件，并选择与方言匹配的子目录
	goose.SetBaseFS(migrations.FS())
	dir := migrations.DialectDir(dialect)

	currentVersion, err := goose.GetDBVersion(db)
	if err != nil {
		logger.Info("[db migration] 初始化 goose 迁移表...")
		currentVersion = 0
	} else {
		logger.Info(fmt.Sprintf("[db migration] 当前数据库版本: %d", currentVersion))
	}

	// 使用 UpByOne 循环执行，便于逐条记录
	for {
		err := goose.UpByOne(db, dir)
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
	if err := RunGooseMigrations(sqlDB, dialect, logger); err != nil {
		return fmt.Errorf("[db migration] goose 迁移失败: %w", err)
	}
	return nil
}

// Stop 迁移服务无需停止操作。
func (receiver *Server) Stop() error {
	return nil
}
