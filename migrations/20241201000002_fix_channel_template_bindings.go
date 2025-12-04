package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upFixChannelTemplateBindings, downFixChannelTemplateBindings)
}

func upFixChannelTemplateBindings(ctx context.Context, tx *sql.Tx) error {
	log.Println("Fixing channel_template_bindings table...")

	// 检查表是否存在
	var tableExists int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'channel_template_bindings'
	`).Scan(&tableExists)
	if err != nil || tableExists == 0 {
		log.Println("Table channel_template_bindings does not exist, skipping...")
		return nil
	}

	// 检查并删除 template_binding_id 列
	var columnExists int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		  AND TABLE_NAME = 'channel_template_bindings' 
		  AND COLUMN_NAME = 'template_binding_id'
	`).Scan(&columnExists)
	if err == nil && columnExists > 0 {
		log.Println("Found template_binding_id column, removing...")

		// 删除外键约束（如果存在）
		var constraintName string
		err = tx.QueryRowContext(ctx, `
			SELECT CONSTRAINT_NAME 
			FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE 
			WHERE TABLE_SCHEMA = DATABASE()
			  AND TABLE_NAME = 'channel_template_bindings'
			  AND COLUMN_NAME = 'template_binding_id'
			  AND REFERENCED_TABLE_NAME IS NOT NULL
			LIMIT 1
		`).Scan(&constraintName)
		if err == nil && constraintName != "" {
			log.Printf("Dropping foreign key constraint: %s", constraintName)
			_, _ = tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE channel_template_bindings DROP FOREIGN KEY %s", constraintName))
		}

		// 删除列
		_, err = tx.ExecContext(ctx, "ALTER TABLE channel_template_bindings DROP COLUMN template_binding_id")
		if err != nil {
			log.Printf("Warning: Failed to drop template_binding_id column: %v", err)
		}
	}

	// 删除可能存在的外键约束以便清理无效数据
	foreignKeys := []string{
		"fk_channel_template_bindings_channel",
		"fk_channel_template_bindings_provider_template",
		"fk_channel_template_bindings_provider_account",
	}

	for _, fk := range foreignKeys {
		var count int
		tx.QueryRowContext(ctx, `
			SELECT COUNT(*) 
			FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS 
			WHERE TABLE_SCHEMA = DATABASE()
			  AND TABLE_NAME = 'channel_template_bindings'
			  AND CONSTRAINT_NAME = ?
			  AND CONSTRAINT_TYPE = 'FOREIGN KEY'
		`, fk).Scan(&count)

		if count > 0 {
			log.Printf("Dropping foreign key: %s", fk)
			_, _ = tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE channel_template_bindings DROP FOREIGN KEY %s", fk))
		}
	}

	// 清理无效数据
	log.Println("Cleaning invalid data...")

	// 删除引用不存在的 provider_template_id 的记录
	result, _ := tx.ExecContext(ctx, `
		DELETE ctb FROM channel_template_bindings ctb
		LEFT JOIN provider_templates pt ON ctb.provider_template_id = pt.id
		WHERE ctb.provider_template_id IS NOT NULL 
		  AND pt.id IS NULL
	`)
	if rows, _ := result.RowsAffected(); rows > 0 {
		log.Printf("Deleted %d records with invalid provider_template_id", rows)
	}

	// 删除引用不存在的 provider_id 的记录
	result, _ = tx.ExecContext(ctx, `
		DELETE ctb FROM channel_template_bindings ctb
		LEFT JOIN provider_accounts pa ON ctb.provider_id = pa.id
		WHERE ctb.provider_id IS NOT NULL 
		  AND pa.id IS NULL
	`)
	if rows, _ := result.RowsAffected(); rows > 0 {
		log.Printf("Deleted %d records with invalid provider_id", rows)
	}

	// 删除引用不存在的 channel_id 的记录
	result, _ = tx.ExecContext(ctx, `
		DELETE ctb FROM channel_template_bindings ctb
		LEFT JOIN channels c ON ctb.channel_id = c.id
		WHERE ctb.channel_id IS NOT NULL 
		  AND c.id IS NULL
	`)
	if rows, _ := result.RowsAffected(); rows > 0 {
		log.Printf("Deleted %d records with invalid channel_id", rows)
	}

	log.Println("Successfully fixed channel_template_bindings table")
	return nil
}

func downFixChannelTemplateBindings(ctx context.Context, tx *sql.Tx) error {
	// 回滚：不恢复已删除的无效数据
	return nil
}
