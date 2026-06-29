// Package migrations 以 embed.FS 内嵌按数据库方言分目录的 SQL 迁移文件。
//
// 目录结构：
//
//	migrations/mysql/*.sql   MySQL 方言
//	migrations/pgsql/*.sql   PostgreSQL 方言
//	migrations/sqlite/*.sql  SQLite 方言
//
// 迁移由 goose 在运行时按 database.driver 选择对应子目录执行，
// 详见 migration.RunGooseMigrations。
package migrations

import "embed"

//go:embed mysql/*.sql pgsql/*.sql sqlite/*.sql
var embedFS embed.FS

// FS 返回内嵌的迁移文件系统。
func FS() embed.FS {
	return embedFS
}

// DialectDir 将数据库驱动名称映射为内嵌迁移文件的子目录。
func DialectDir(driver string) string {
	switch driver {
	case "postgres", "postgresql", "pgsql":
		return "pgsql"
	case "mysql", "mariadb":
		return "mysql"
	case "sqlite", "sqlite3":
		return "sqlite"
	default:
		if driver == "" {
			return "mysql"
		}
		return driver
	}
}
