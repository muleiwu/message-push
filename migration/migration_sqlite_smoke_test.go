package migration

import (
	"database/sql"
	"testing"

	migrationFS "cnb.cool/mliev/push/message-push/migrations"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

// TestSQLiteMigrationsSmoke 在临时 SQLite 上跑完整迁移，校验 SQL 文件与 goose 接线。
func TestSQLiteMigrationsSmoke(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/smoke.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	goose.SetBaseFS(migrationFS.FS())
	if err := goose.UpTo(sqlDB, migrationFS.DialectDir("sqlite"), 20260714000010); err != nil {
		t.Fatalf("run migrations before status normalization: %v", err)
	}

	legacyRows := []string{
		`INSERT INTO applications (app_id, app_secret, app_name, status) VALUES ('legacy-app', 'secret', 'Legacy App', 2)`,
		`INSERT INTO provider_accounts (account_code, account_name, provider_code, provider_type, config, status) VALUES ('legacy-provider', 'Legacy Provider', 'smtp', 'email', '{}', 2)`,
		`INSERT INTO channels (name, type, status) VALUES ('Legacy Channel', 'email', 2)`,
		`INSERT INTO channel_template_bindings (channel_id, provider_template_id, provider_id, status) VALUES (1, 1, 1, 2)`,
	}
	for _, statement := range legacyRows {
		if _, err := sqlDB.Exec(statement); err != nil {
			t.Fatalf("seed legacy status: %v", err)
		}
	}

	if err := RunGooseMigrations(sqlDB, "sqlite", nil); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	assertStatus(t, sqlDB, "applications", 1, 0)
	assertStatus(t, sqlDB, "provider_accounts", 1, 0)
	assertStatus(t, sqlDB, "channels", 1, 0)
	assertStatus(t, sqlDB, "channel_template_bindings", 1, 0)

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

func assertStatus(t *testing.T, db *sql.DB, table string, id int, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("SELECT status FROM "+table+" WHERE id = ?", id).Scan(&got); err != nil {
		t.Fatalf("query %s status: %v", table, err)
	}
	if got != want {
		t.Errorf("%s status = %d, want %d", table, got, want)
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
