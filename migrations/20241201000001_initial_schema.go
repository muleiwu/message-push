package migrations

import (
	"context"
	"database/sql"
	"log"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upInitialSchema, downInitialSchema)
}

func upInitialSchema(ctx context.Context, tx *sql.Tx) error {
	// 使用外部设置的数据库连接
	// 安装时由安装控制器设置，运行时由迁移服务设置
	db := getExternalDB()

	if db == nil {
		log.Println("[initial_schema] 外部数据库连接未设置，跳过 AutoMigrate")
		return nil
	}

	log.Println("[initial_schema] 执行 GORM AutoMigrate...")

	// 使用 GORM AutoMigrate 创建初始表结构
	models := helper.GetConfig().Get("database.migration", []any{}).([]any)
	if err := db.AutoMigrate(models...); err != nil {
		return err
	}

	log.Println("[initial_schema] AutoMigrate 完成")
	return nil
}

func downInitialSchema(ctx context.Context, tx *sql.Tx) error {
	// 回滚：不删除表，保留数据
	return nil
}
