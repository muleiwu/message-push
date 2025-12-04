package migrations

import (
	"context"
	"database/sql"
	"log"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upMigrateProviderMsgID, downMigrateProviderMsgID)
}

func upMigrateProviderMsgID(ctx context.Context, tx *sql.Tx) error {
	log.Println("Migrating provider_msg_id from push_tasks to push_logs...")

	// 检查 push_tasks 表是否存在
	var tableExists int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'push_tasks'
	`).Scan(&tableExists)
	if err != nil || tableExists == 0 {
		log.Println("Table push_tasks does not exist, skipping...")
		return nil
	}

	// 检查 push_tasks 表是否有 provider_msg_id 列
	var columnExists int
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() 
		  AND TABLE_NAME = 'push_tasks' 
		  AND COLUMN_NAME = 'provider_msg_id'
	`).Scan(&columnExists)
	if err != nil || columnExists == 0 {
		log.Println("Column provider_msg_id does not exist in push_tasks, skipping...")
		return nil
	}

	// 检查 push_logs 表是否存在
	var logsTableExists int
	tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'push_logs'
	`).Scan(&logsTableExists)

	if logsTableExists > 0 {
		// 检查 push_logs 表是否已有 provider_msg_id 列
		var logsColumnExists int
		tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS 
			WHERE TABLE_SCHEMA = DATABASE() 
			  AND TABLE_NAME = 'push_logs' 
			  AND COLUMN_NAME = 'provider_msg_id'
		`).Scan(&logsColumnExists)

		if logsColumnExists == 0 {
			log.Println("Adding provider_msg_id column to push_logs...")
			_, err = tx.ExecContext(ctx, `
				ALTER TABLE push_logs 
				ADD COLUMN provider_msg_id VARCHAR(100) DEFAULT NULL COMMENT '服务商返回的消息ID'
			`)
			if err != nil {
				return err
			}

			// 添加索引
			log.Println("Adding index idx_provider_msg_id on push_logs...")
			_, _ = tx.ExecContext(ctx, "ALTER TABLE push_logs ADD INDEX idx_provider_msg_id (provider_msg_id)")
		}

		// 迁移数据
		log.Println("Migrating provider_msg_id data...")
		result, _ := tx.ExecContext(ctx, `
			UPDATE push_logs pl
			INNER JOIN (
				SELECT task_id, provider_msg_id 
				FROM push_tasks 
				WHERE provider_msg_id IS NOT NULL AND provider_msg_id != ''
			) pt ON pl.task_id = pt.task_id
			INNER JOIN (
				SELECT task_id, MAX(id) as max_id
				FROM push_logs
				GROUP BY task_id
			) latest ON pl.task_id = latest.task_id AND pl.id = latest.max_id
			SET pl.provider_msg_id = pt.provider_msg_id
			WHERE pl.provider_msg_id IS NULL OR pl.provider_msg_id = ''
		`)
		if rows, _ := result.RowsAffected(); rows > 0 {
			log.Printf("Migrated %d push_log records with provider_msg_id", rows)
		}
	} else {
		log.Println("Table push_logs does not exist, skipping data migration...")
	}

	// 清理 push_tasks 表的 provider_msg_id
	// 删除 push_tasks 表的 provider_msg_id 索引（如果存在）
	var indexExists int
	tx.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM INFORMATION_SCHEMA.STATISTICS 
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'push_tasks'
		  AND INDEX_NAME = 'idx_provider_msg_id'
	`).Scan(&indexExists)

	if indexExists > 0 {
		log.Println("Dropping index idx_provider_msg_id from push_tasks...")
		_, _ = tx.ExecContext(ctx, "ALTER TABLE push_tasks DROP INDEX idx_provider_msg_id")
	}

	// 删除 push_tasks 表的 provider_msg_id 列
	log.Println("Dropping provider_msg_id column from push_tasks...")
	_, err = tx.ExecContext(ctx, "ALTER TABLE push_tasks DROP COLUMN provider_msg_id")
	if err != nil {
		return err
	}

	log.Println("Successfully migrated provider_msg_id from push_tasks to push_logs")
	return nil
}

func downMigrateProviderMsgID(ctx context.Context, tx *sql.Tx) error {
	// 回滚：重新添加 provider_msg_id 到 push_tasks
	log.Println("Adding provider_msg_id column back to push_tasks table...")
	_, err := tx.ExecContext(ctx, `
		ALTER TABLE push_tasks 
		ADD COLUMN provider_msg_id VARCHAR(100) DEFAULT NULL COMMENT '服务商返回的消息ID'
	`)
	return err
}
