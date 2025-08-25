package interfaces

import (
	"gorm.io/gorm"
)

type DatabaseInterface interface {
	// 基础连接方法
	GetClient() *gorm.DB
	SetClient(db *gorm.DB)
	Close() error

	// 数据库操作
	Create(value interface{}) error
	Save(value interface{}) error
	Delete(value interface{}, where ...interface{}) error
	Find(dest interface{}, conds ...interface{}) error
	First(dest interface{}, conds ...interface{}) error
	Last(dest interface{}, conds ...interface{}) error
	Take(dest interface{}, conds ...interface{}) error

	// 条件查询
	Where(query interface{}, args ...interface{}) DatabaseInterface
	Select(query interface{}, args ...interface{}) DatabaseInterface
	Omit(columns ...string) DatabaseInterface
	Order(value interface{}) DatabaseInterface
	Limit(limit int) DatabaseInterface
	Offset(offset int) DatabaseInterface
	Group(name string) DatabaseInterface
	Having(query interface{}, args ...interface{}) DatabaseInterface
	Joins(query string, args ...interface{}) DatabaseInterface

	// 聚合函数
	Count(count *int64) error
	Sum(field string, result interface{}) error
	Avg(field string, result interface{}) error
	Max(field string, result interface{}) error
	Min(field string, result interface{}) error

	// 更新操作
	Update(column string, value interface{}) error
	Updates(values interface{}) error
	UpdateColumn(column string, value interface{}) error
	UpdateColumns(values interface{}) error

	// 批量操作
	CreateInBatches(value interface{}, batchSize int) error
	FindInBatches(dest interface{}, batchSize int, fc func(tx DatabaseInterface, batch int) error) error

	// 事务操作
	Begin() DatabaseInterface
	Commit() error
	Rollback() error
	Transaction(fc func(tx DatabaseInterface) error) error

	// 原生SQL
	Raw(sql string, values ...interface{}) DatabaseInterface
	Exec(sql string, values ...interface{}) error

	// 模型操作
	Model(value interface{}) DatabaseInterface
	Table(name string, args ...interface{}) DatabaseInterface

	// 关联操作
	Preload(query string, args ...interface{}) DatabaseInterface
	Association(column string) *gorm.Association

	// 错误处理
	Error() error
	RowsAffected() int64

	// 迁移
	AutoMigrate(dst ...interface{}) error
	Migrator() gorm.Migrator

	// 作用域
	Scopes(funcs ...func(*gorm.DB) *gorm.DB) DatabaseInterface

	// 会话
	Session(config *gorm.Session) DatabaseInterface

	// 调试
	Debug() DatabaseInterface
}
