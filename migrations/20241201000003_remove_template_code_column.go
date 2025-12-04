package migrations

import (
	"context"
	"database/sql"
	"log"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upRemoveTemplateCodeColumn, downRemoveTemplateCodeColumn)
}

func upRemoveTemplateCodeColumn(ctx context.Context, tx *sql.Tx) error {
	log.Println("Removing template_code column from message_templates table...")

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

	// 检查 template_code 列是否存在
	var columnExists int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		  AND TABLE_NAME = 'message_templates' 
		  AND COLUMN_NAME = 'template_code'
	`).Scan(&columnExists)
	if err != nil || columnExists == 0 {
		log.Println("Column template_code does not exist in message_templates, skipping...")
		return nil
	}

	// 删除唯一索引 uk_template_code（如果存在）
	var indexExists int
	tx.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM INFORMATION_SCHEMA.STATISTICS 
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'message_templates'
		  AND INDEX_NAME = 'uk_template_code'
	`).Scan(&indexExists)

	if indexExists > 0 {
		log.Println("Dropping unique index uk_template_code...")
		_, _ = tx.ExecContext(ctx, "ALTER TABLE message_templates DROP INDEX uk_template_code")
	}

	// 删除 template_code 列
	log.Println("Dropping template_code column...")
	_, err = tx.ExecContext(ctx, "ALTER TABLE message_templates DROP COLUMN template_code")
	if err != nil {
		return err
	}

	log.Println("Successfully removed template_code column from message_templates table")
	return nil
}

func downRemoveTemplateCodeColumn(ctx context.Context, tx *sql.Tx) error {
	// 回滚：重新添加 template_code 列
	log.Println("Adding template_code column back to message_templates table...")
	_, err := tx.ExecContext(ctx, `
		ALTER TABLE message_templates 
		ADD COLUMN template_code VARCHAR(100) DEFAULT NULL COMMENT '模板代码'
	`)
	return err
}
