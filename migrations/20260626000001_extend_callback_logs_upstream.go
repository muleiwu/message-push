package migrations

import (
	"context"
	"database/sql"
	"log"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationContext(upExtendCallbackLogsUpstream, downExtendCallbackLogsUpstream)
}

// upExtendCallbackLogsUpstream 扩展 callback_logs，统一存储下行回执与上行短信
func upExtendCallbackLogsUpstream(ctx context.Context, tx *sql.Tx) error {
	log.Println("Extending callback_logs for upstream SMS...")

	var tableExists int
	tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'callback_logs'
	`).Scan(&tableExists)
	if tableExists == 0 {
		log.Println("callback_logs not found, skipping...")
		return nil
	}

	// 新增列
	columns := []struct {
		name string
		ddl  string
	}{
		{"type", `ALTER TABLE callback_logs ADD COLUMN type VARCHAR(20) NOT NULL DEFAULT 'report' COMMENT '回调类型: report=下行回执, upstream=上行短信' AFTER id`},
		{"mobile", `ALTER TABLE callback_logs ADD COLUMN mobile VARCHAR(20) DEFAULT NULL COMMENT '手机号(回执=接收方,上行=发送方)' AFTER provider_id`},
		{"content", `ALTER TABLE callback_logs ADD COLUMN content TEXT DEFAULT NULL COMMENT '上行短信回复内容' AFTER mobile`},
	}
	for _, col := range columns {
		var exists int
		tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'callback_logs' AND COLUMN_NAME = ?
		`, col.name).Scan(&exists)
		if exists == 0 {
			if _, err := tx.ExecContext(ctx, col.ddl); err != nil {
				return err
			}
			log.Printf("Added column callback_logs.%s", col.name)
		}
	}

	// 新增索引
	indexes := []struct {
		name string
		ddl  string
	}{
		{"idx_type", "ALTER TABLE callback_logs ADD INDEX idx_type (type)"},
		{"idx_mobile", "ALTER TABLE callback_logs ADD INDEX idx_mobile (mobile)"},
	}
	for _, idx := range indexes {
		var exists int
		tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM INFORMATION_SCHEMA.STATISTICS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'callback_logs' AND INDEX_NAME = ?
		`, idx.name).Scan(&exists)
		if exists == 0 {
			if _, err := tx.ExecContext(ctx, idx.ddl); err != nil {
				return err
			}
			log.Printf("Added index callback_logs.%s", idx.name)
		}
	}

	log.Println("Successfully extended callback_logs for upstream SMS")
	return nil
}

// downExtendCallbackLogsUpstream 回滚：删除新增列与索引
func downExtendCallbackLogsUpstream(ctx context.Context, tx *sql.Tx) error {
	_, _ = tx.ExecContext(ctx, "ALTER TABLE callback_logs DROP INDEX idx_type")
	_, _ = tx.ExecContext(ctx, "ALTER TABLE callback_logs DROP INDEX idx_mobile")
	_, _ = tx.ExecContext(ctx, "ALTER TABLE callback_logs DROP COLUMN content")
	_, _ = tx.ExecContext(ctx, "ALTER TABLE callback_logs DROP COLUMN mobile")
	_, _ = tx.ExecContext(ctx, "ALTER TABLE callback_logs DROP COLUMN type")
	return nil
}
