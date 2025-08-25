package impl

import (
	"errors"
	"fmt"
	"sync"

	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Database struct {
	db          *gorm.DB
	initialized bool
	initOnce    sync.Once
	initError   error
	helper      interfaces.HelperInterface
}

func getMySQLDSN(host string, port int, username string, password string, dbName string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		username,
		password,
		host,
		port,
		dbName)
}

func getPostgreSQLDSN(host string, port int, username string, password string, dbName string) string {
	return fmt.Sprintf("user=%s password=%s host=%s port=%d dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		username,
		password,
		host,
		port,
		dbName)
}
func getSqliteSQLDSN(host string, port int, username string, password string, dbName string) string {
	return host
}

func NewDatabase(helper interfaces.HelperInterface, driver string, host string, port int, dbName string, username string, password string) *Database {
	d := &Database{
		helper: helper,
	}
	d.initOnce.Do(func() {

		var dialector gorm.Dialector
		if driver == "postgresql" {
			dialector = postgres.New(postgres.Config{
				DSN:                  getPostgreSQLDSN(host, port, username, password, dbName),
				PreferSimpleProtocol: true,
			})
		} else if driver == "mysql" {
			dialector = mysql.Open(getMySQLDSN(host, port, username, password, dbName))
		} else if driver == "sqlite" {
			dialector = sqlite.Open(getSqliteSQLDSN(host, port, username, password, dbName))
		} else if driver == "memory" {
			dialector = sqlite.Open(":memory:")
		} else {
			d.initError = errors.New("unsupported database driver")
			return
		}

		var err error
		d.db, err = gorm.Open(dialector, &gorm.Config{})
		if err != nil {
			d.initError = err
			return
		}

		d.initialized = true
	})

	return d
}

// 基础连接方法
func (d *Database) GetClient() *gorm.DB {
	return d.db
}

func (d *Database) SetClient(db *gorm.DB) {
	d.db = db
}

func (d *Database) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// 数据库操作
func (d *Database) Create(value interface{}) error {
	return d.db.Create(value).Error
}

func (d *Database) Save(value interface{}) error {
	return d.db.Save(value).Error
}

func (d *Database) Delete(value interface{}, where ...interface{}) error {
	return d.db.Delete(value, where...).Error
}

func (d *Database) Find(dest interface{}, conds ...interface{}) error {
	return d.db.Find(dest, conds...).Error
}

func (d *Database) First(dest interface{}, conds ...interface{}) error {
	return d.db.First(dest, conds...).Error
}

func (d *Database) Last(dest interface{}, conds ...interface{}) error {
	return d.db.Last(dest, conds...).Error
}

func (d *Database) Take(dest interface{}, conds ...interface{}) error {
	return d.db.Take(dest, conds...).Error
}

// 条件查询
func (d *Database) Where(query interface{}, args ...interface{}) interfaces.DatabaseInterface {
	return &Database{db: d.db.Where(query, args...)}
}

func (d *Database) Select(query interface{}, args ...interface{}) interfaces.DatabaseInterface {
	return &Database{db: d.db.Select(query, args...)}
}

func (d *Database) Omit(columns ...string) interfaces.DatabaseInterface {
	return &Database{db: d.db.Omit(columns...)}
}

func (d *Database) Order(value interface{}) interfaces.DatabaseInterface {
	return &Database{db: d.db.Order(value)}
}

func (d *Database) Limit(limit int) interfaces.DatabaseInterface {
	return &Database{db: d.db.Limit(limit)}
}

func (d *Database) Offset(offset int) interfaces.DatabaseInterface {
	return &Database{db: d.db.Offset(offset)}
}

func (d *Database) Group(name string) interfaces.DatabaseInterface {
	return &Database{db: d.db.Group(name)}
}

func (d *Database) Having(query interface{}, args ...interface{}) interfaces.DatabaseInterface {
	return &Database{db: d.db.Having(query, args...)}
}

func (d *Database) Joins(query string, args ...interface{}) interfaces.DatabaseInterface {
	return &Database{db: d.db.Joins(query, args...)}
}

// 聚合函数
func (d *Database) Count(count *int64) error {
	return d.db.Count(count).Error
}

func (d *Database) Sum(field string, result interface{}) error {
	return d.db.Select(fmt.Sprintf("SUM(%s)", field)).Row().Scan(result)
}

func (d *Database) Avg(field string, result interface{}) error {
	return d.db.Select(fmt.Sprintf("AVG(%s)", field)).Row().Scan(result)
}

func (d *Database) Max(field string, result interface{}) error {
	return d.db.Select(fmt.Sprintf("MAX(%s)", field)).Row().Scan(result)
}

func (d *Database) Min(field string, result interface{}) error {
	return d.db.Select(fmt.Sprintf("MIN(%s)", field)).Row().Scan(result)
}

// 更新操作
func (d *Database) Update(column string, value interface{}) error {
	return d.db.Update(column, value).Error
}

func (d *Database) Updates(values interface{}) error {
	return d.db.Updates(values).Error
}

func (d *Database) UpdateColumn(column string, value interface{}) error {
	return d.db.UpdateColumn(column, value).Error
}

func (d *Database) UpdateColumns(values interface{}) error {
	return d.db.UpdateColumns(values).Error
}

// 批量操作
func (d *Database) CreateInBatches(value interface{}, batchSize int) error {
	return d.db.CreateInBatches(value, batchSize).Error
}

func (d *Database) FindInBatches(dest interface{}, batchSize int, fc func(tx interfaces.DatabaseInterface, batch int) error) error {
	return d.db.FindInBatches(dest, batchSize, func(tx *gorm.DB, batch int) error {
		return fc(&Database{db: tx}, batch)
	}).Error
}

// 事务操作
func (d *Database) Begin() interfaces.DatabaseInterface {
	return &Database{db: d.db.Begin()}
}

func (d *Database) Commit() error {
	return d.db.Commit().Error
}

func (d *Database) Rollback() error {
	return d.db.Rollback().Error
}

func (d *Database) Transaction(fc func(tx interfaces.DatabaseInterface) error) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		return fc(&Database{db: tx})
	})
}

// 原生SQL
func (d *Database) Raw(sql string, values ...interface{}) interfaces.DatabaseInterface {
	return &Database{db: d.db.Raw(sql, values...)}
}

func (d *Database) Exec(sql string, values ...interface{}) error {
	return d.db.Exec(sql, values...).Error
}

// 模型操作
func (d *Database) Model(value interface{}) interfaces.DatabaseInterface {
	return &Database{db: d.db.Model(value)}
}

func (d *Database) Table(name string, args ...interface{}) interfaces.DatabaseInterface {
	return &Database{db: d.db.Table(name, args...)}
}

// 关联操作
func (d *Database) Preload(query string, args ...interface{}) interfaces.DatabaseInterface {
	return &Database{db: d.db.Preload(query, args...)}
}

func (d *Database) Association(column string) *gorm.Association {
	return d.db.Association(column)
}

// 错误处理
func (d *Database) Error() error {
	return d.db.Error
}

func (d *Database) RowsAffected() int64 {
	return d.db.RowsAffected
}

// 迁移
func (d *Database) AutoMigrate(dst ...interface{}) error {
	return d.db.AutoMigrate(dst...)
}

func (d *Database) Migrator() gorm.Migrator {
	return d.db.Migrator()
}

// 作用域
func (d *Database) Scopes(funcs ...func(*gorm.DB) *gorm.DB) interfaces.DatabaseInterface {
	return &Database{db: d.db.Scopes(funcs...)}
}

// 会话
func (d *Database) Session(config *gorm.Session) interfaces.DatabaseInterface {
	return &Database{db: d.db.Session(config)}
}

// 调试
func (d *Database) Debug() interfaces.DatabaseInterface {
	return &Database{db: d.db.Debug()}
}
