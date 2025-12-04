package migrations

import (
	"sync"

	"gorm.io/gorm"
)

var (
	// externalDB 用于安装时传入的外部数据库连接
	externalDB   *gorm.DB
	externalDBMu sync.RWMutex
)

// SetExternalDB 设置外部数据库连接（供安装控制器和迁移服务使用）
func SetExternalDB(db *gorm.DB) {
	externalDBMu.Lock()
	defer externalDBMu.Unlock()
	externalDB = db
}

// ClearExternalDB 清除外部数据库连接
func ClearExternalDB() {
	externalDBMu.Lock()
	defer externalDBMu.Unlock()
	externalDB = nil
}

// getExternalDB 获取外部数据库连接
func getExternalDB() *gorm.DB {
	externalDBMu.RLock()
	defer externalDBMu.RUnlock()
	return externalDB
}
