package migrations

import (
	"context"
	"database/sql"
	"log"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upAddContentType, downAddContentType)
}

func upAddContentType(ctx context.Context, tx *sql.Tx) error {
	log.Println("Adding content_type column to template tables...")

	// 为 message_templates 表添加 content_type 字段
	var tableExists int
	tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'message_templates'
	`).Scan(&tableExists)

	if tableExists > 0 {
		var columnExists int
		tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
			WHERE TABLE_SCHEMA = DATABASE() 
			  AND TABLE_NAME = 'message_templates' 
			  AND COLUMN_NAME = 'content_type'
		`).Scan(&columnExists)

		if columnExists == 0 {
			log.Println("Adding content_type column to message_templates...")

			// 检查是否有 message_type 列来决定插入位置
			var hasMessageType int
			tx.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
				WHERE TABLE_SCHEMA = DATABASE() 
				  AND TABLE_NAME = 'message_templates' 
				  AND COLUMN_NAME = 'message_type'
			`).Scan(&hasMessageType)

			var sql string
			if hasMessageType > 0 {
				sql = `ALTER TABLE message_templates ADD COLUMN content_type VARCHAR(20) DEFAULT 'text' COMMENT '内容类型：text=纯文本, html=HTML富文本, markdown=Markdown' AFTER message_type`
			} else {
				sql = `ALTER TABLE message_templates ADD COLUMN content_type VARCHAR(20) DEFAULT 'text' COMMENT '内容类型：text=纯文本, html=HTML富文本, markdown=Markdown' AFTER template_name`
			}
			_, err := tx.ExecContext(ctx, sql)
			if err != nil {
				return err
			}
		} else {
			log.Println("Column content_type already exists in message_templates, skipping...")
		}
	}

	// 为 provider_templates 表添加 content_type 字段
	tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'provider_templates'
	`).Scan(&tableExists)

	if tableExists > 0 {
		var columnExists int
		tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
			WHERE TABLE_SCHEMA = DATABASE() 
			  AND TABLE_NAME = 'provider_templates' 
			  AND COLUMN_NAME = 'content_type'
		`).Scan(&columnExists)

		if columnExists == 0 {
			log.Println("Adding content_type column to provider_templates...")
			_, err := tx.ExecContext(ctx, `
				ALTER TABLE provider_templates 
				ADD COLUMN content_type VARCHAR(20) DEFAULT 'text' COMMENT '内容类型：text=纯文本, html=HTML富文本, markdown=Markdown' AFTER template_name
			`)
			if err != nil {
				return err
			}
		} else {
			log.Println("Column content_type already exists in provider_templates, skipping...")
		}
	}

	log.Println("Successfully added content_type column to template tables")
	return nil
}

func downAddContentType(ctx context.Context, tx *sql.Tx) error {
	// 回滚：删除 content_type 列
	log.Println("Removing content_type column from template tables...")

	_, _ = tx.ExecContext(ctx, "ALTER TABLE message_templates DROP COLUMN content_type")
	_, _ = tx.ExecContext(ctx, "ALTER TABLE provider_templates DROP COLUMN content_type")

	return nil
}
