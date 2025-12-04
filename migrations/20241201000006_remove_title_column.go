package migrations

import (
	"context"
	"database/sql"
	"log"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upRemoveTitleColumn, downRemoveTitleColumn)
}

func upRemoveTitleColumn(ctx context.Context, tx *sql.Tx) error {
	log.Println("Removing title column from push_tasks table...")

	// 检查表是否存在
	var tableExists int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'push_tasks'
	`).Scan(&tableExists)
	if err != nil || tableExists == 0 {
		log.Println("Table push_tasks does not exist, skipping...")
		return nil
	}

	// 检查 title 列是否存在
	var columnExists int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		  AND TABLE_NAME = 'push_tasks' 
		  AND COLUMN_NAME = 'title'
	`).Scan(&columnExists)
	if err != nil || columnExists == 0 {
		log.Println("Column title does not exist in push_tasks, skipping...")
		return nil
	}

	// 删除 title 列
	log.Println("Dropping title column...")
	_, err = tx.ExecContext(ctx, "ALTER TABLE push_tasks DROP COLUMN title")
	if err != nil {
		return err
	}

	log.Println("Successfully removed title column from push_tasks table")
	return nil
}

func downRemoveTitleColumn(ctx context.Context, tx *sql.Tx) error {
	// 回滚：重新添加 title 列
	log.Println("Adding title column back to push_tasks table...")
	_, err := tx.ExecContext(ctx, `
		ALTER TABLE push_tasks 
		ADD COLUMN title VARCHAR(200) DEFAULT NULL COMMENT '消息标题（邮件主题等）'
	`)
	return err
}
