package migration

import (
	"database/sql"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestSQLiteMigrationsSmoke 在临时 SQLite 上跑完整 0001~0009 迁移，校验 SQL 文件与 goose 接线。
func TestSQLiteMigrationsSmoke(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/smoke.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	if err := RunGooseMigrations(sqlDB, "sqlite", nil); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	// 校验最终 schema：列应处于演进后的最终状态
	assertNoColumn(t, sqlDB, "push_tasks", "title")
	assertNoColumn(t, sqlDB, "push_tasks", "content")
	assertNoColumn(t, sqlDB, "push_tasks", "provider_msg_id")
	assertNoColumn(t, sqlDB, "message_templates", "template_code")
	assertNoColumn(t, sqlDB, "message_templates", "message_type")
	assertHasColumn(t, sqlDB, "message_templates", "content_type")
	assertHasColumn(t, sqlDB, "provider_templates", "content_type")
	assertHasColumn(t, sqlDB, "push_logs", "provider_msg_id")
	assertHasColumn(t, sqlDB, "callback_logs", "type")
	assertHasColumn(t, sqlDB, "callback_logs", "mobile")
	assertHasColumn(t, sqlDB, "callback_logs", "content")
	assertNoColumn(t, sqlDB, "channel_template_bindings", "template_binding_id")

	// 重复执行应为幂等（无新版本）
	if err := RunGooseMigrations(sqlDB, "sqlite", nil); err != nil {
		t.Fatalf("rerun migrations: %v", err)
	}
}

func hasColumn(t *testing.T, db *sql.DB, table, col string) bool {
	t.Helper()
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("pragma table_info(%s): %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == col {
			return true
		}
	}
	return false
}

func assertHasColumn(t *testing.T, db *sql.DB, table, col string) {
	t.Helper()
	if !hasColumn(t, db, table, col) {
		t.Errorf("expected column %s.%s to exist", table, col)
	}
}

func assertNoColumn(t *testing.T, db *sql.DB, table, col string) {
	t.Helper()
	if hasColumn(t, db, table, col) {
		t.Errorf("expected column %s.%s to be absent", table, col)
	}
}
