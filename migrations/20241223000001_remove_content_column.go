package migrations

import (
	"context"
	"database/sql"
	"log"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upRemoveContentColumn, downRemoveContentColumn)
}

func upRemoveContentColumn(ctx context.Context, tx *sql.Tx) error {
	log.Println("Removing content column from push_tasks table...")

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

	// 检查 content 列是否存在
	var columnExists int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		  AND TABLE_NAME = 'push_tasks' 
		  AND COLUMN_NAME = 'content'
	`).Scan(&columnExists)
	if err != nil || columnExists == 0 {
		log.Println("Column content does not exist in push_tasks, skipping...")
		return nil
	}

	// 删除 content 列
	log.Println("Dropping content column...")
	_, err = tx.ExecContext(ctx, "ALTER TABLE push_tasks DROP COLUMN content")
	if err != nil {
		return err
	}

	log.Println("Successfully removed content column from push_tasks table")
	return nil
}

func downRemoveContentColumn(ctx context.Context, tx *sql.Tx) error {
	// 回滚：重新添加 content 列
	log.Println("Adding content column back to push_tasks table...")

	// 添加 content 列（在 receiver 之后）
	_, err := tx.ExecContext(ctx, `
		ALTER TABLE push_tasks 
		ADD COLUMN content TEXT COMMENT '内容（直接发送或模板渲染后内容）' AFTER receiver
	`)
	if err != nil {
		return err
	}

	log.Println("Successfully added content column back to push_tasks table")
	return nil
}

