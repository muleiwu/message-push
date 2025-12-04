package migrations

import (
	"context"
	"database/sql"
	"log"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upRemoveMessageType, downRemoveMessageType)
}

func upRemoveMessageType(ctx context.Context, tx *sql.Tx) error {
	log.Println("Removing message_type column from message_templates table...")

	// 检查表是否存在
	var tableExists int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'message_templates'
	`).Scan(&tableExists)
	if err != nil || tableExists == 0 {
		log.Println("Table message_templates does not exist, skipping...")
		return nil
	}

	// 检查 message_type 列是否存在
	var columnExists int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		  AND TABLE_NAME = 'message_templates' 
		  AND COLUMN_NAME = 'message_type'
	`).Scan(&columnExists)
	if err != nil || columnExists == 0 {
		log.Println("Column message_type does not exist in message_templates, skipping...")
		return nil
	}

	// 删除包含 message_type 的复合索引 idx_type_status（如果存在）
	var indexExists int
	tx.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM INFORMATION_SCHEMA.STATISTICS 
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'message_templates'
		  AND INDEX_NAME = 'idx_type_status'
	`).Scan(&indexExists)

	if indexExists > 0 {
		log.Println("Dropping index idx_type_status...")
		_, err = tx.ExecContext(ctx, "ALTER TABLE message_templates DROP INDEX idx_type_status")
		if err != nil {
			log.Printf("Warning: Failed to drop index idx_type_status: %v", err)
		}
	}

	// 删除 message_type 列
	log.Println("Dropping message_type column...")
	_, err = tx.ExecContext(ctx, "ALTER TABLE message_templates DROP COLUMN message_type")
	if err != nil {
		return err
	}

	// 创建新的 idx_status 索引（如果不存在）
	var statusIndexExists int
	tx.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM INFORMATION_SCHEMA.STATISTICS 
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'message_templates'
		  AND INDEX_NAME = 'idx_status'
	`).Scan(&statusIndexExists)

	if statusIndexExists == 0 {
		log.Println("Creating index idx_status...")
		_, err = tx.ExecContext(ctx, "ALTER TABLE message_templates ADD INDEX idx_status (status)")
		if err != nil {
			log.Printf("Warning: Failed to create index idx_status: %v", err)
		}
	}

	log.Println("Successfully removed message_type column from message_templates table")
	return nil
}

func downRemoveMessageType(ctx context.Context, tx *sql.Tx) error {
	// 回滚：重新添加 message_type 列
	log.Println("Adding message_type column back to message_templates table...")

	// 删除 idx_status 索引
	_, _ = tx.ExecContext(ctx, "ALTER TABLE message_templates DROP INDEX idx_status")

	// 添加 message_type 列
	_, err := tx.ExecContext(ctx, `
		ALTER TABLE message_templates 
		ADD COLUMN message_type VARCHAR(20) NOT NULL DEFAULT 'sms' COMMENT '消息类型：sms, email, wechat_work, dingtalk, webhook, push' AFTER template_name
	`)
	if err != nil {
		return err
	}

	// 重新创建复合索引
	_, _ = tx.ExecContext(ctx, "ALTER TABLE message_templates ADD INDEX idx_type_status (message_type, status)")

	return nil
}
