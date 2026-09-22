package migration

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	migrationFS "cnb.cool/mliev/push/message-push/migrations"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

// TestSQLiteMigrationsSmoke 在临时 SQLite 上跑完整迁移，校验 SQL 文件与 goose 接线。
func TestSQLiteMigrationsSmoke(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/smoke.db?_time_format=sqlite&_timezone=UTC"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
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
		`INSERT INTO provider_accounts (account_code, account_name, provider_code, provider_type, config, status) VALUES ('latest-provider', 'Latest Provider', 'smtp', 'email', '{}', 1)`,
		`INSERT INTO channels (name, type, status) VALUES ('Legacy Channel', 'email', 2)`,
		`INSERT INTO channel_template_bindings (channel_id, provider_template_id, provider_id, status) VALUES (1, 1, 1, 2)`,
		`INSERT INTO push_tasks (task_id, app_id, channel_id, message_type, receiver, status) VALUES ('provider-backfill', 'legacy-app', 1, 'email', 'legacy@example.com', 'success')`,
		`INSERT INTO push_logs (task_id, app_id, provider_account_id, status) VALUES ('provider-backfill', 'legacy-app', 1, 'failed')`,
		`INSERT INTO push_logs (task_id, app_id, provider_account_id, status) VALUES ('provider-backfill', 'legacy-app', 2, 'success')`,
		`INSERT INTO app_quota_stats (app_id, stat_date, total_count) VALUES ('legacy-app', '2026-08-04', 1)`,
		`INSERT INTO admin_users (username, password, real_name, email, auth_source, status) VALUES ('mixed-email', 'hash', 'Mixed Email', '  Admin@Example.COM  ', 'local', 1)`,
		`INSERT INTO admin_users (username, password, real_name, email, auth_source, status) VALUES ('blank-email', 'hash', 'Blank Email', '   ', 'local', 1)`,
		`INSERT INTO admin_users (username, password, real_name, email, auth_source, status) VALUES ('legacy-admin-status', 'hash', 'Legacy Admin Status', NULL, 'local', 2)`,
	}
	for _, statement := range legacyRows {
		if _, err := sqlDB.Exec(statement); err != nil {
			t.Fatalf("seed legacy status: %v", err)
		}
	}
	if _, err := sqlDB.Exec(`
		UPDATE applications
		SET created_at = '2026-08-04 10:00:00',
			updated_at = '2026-08-04 18:00:00+08:00'
		WHERE app_id = 'legacy-app'
	`); err != nil {
		t.Fatalf("seed legacy timestamps: %v", err)
	}
	if err := goose.UpTo(sqlDB, migrationFS.DialectDir("sqlite"), preUTCMigrationVersion); err != nil {
		t.Fatalf("run migrations before UTC normalization: %v", err)
	}
	if _, err := sqlDB.Exec(`
		INSERT INTO webhook_logs (
			task_id, app_id, webhook_url, event, status, dedup_key,
			next_attempt_at, locked_until, created_at, updated_at
		) VALUES (
			'offset-webhook', 'legacy-app', 'https://example.test/webhook', 'failed', 'pending', 'offset-webhook',
			'2026-08-04 18:00:00+08:00', NULL, '2026-08-04 10:00:00', '2026-08-04 18:00:00+08:00'
		)
	`); err != nil {
		t.Fatalf("seed legacy webhook timestamps: %v", err)
	}

	if err := RunGooseMigrations(sqlDB, "sqlite", nil); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	assertStatus(t, sqlDB, "applications", 1, 0)
	assertStatus(t, sqlDB, "provider_accounts", 1, 0)
	assertStatus(t, sqlDB, "channels", 1, 0)
	assertStatus(t, sqlDB, "channel_template_bindings", 1, 0)
	assertAdminEmail(t, sqlDB, "mixed-email", "admin@example.com", true)
	assertAdminEmail(t, sqlDB, "blank-email", "", false)
	assertAdminStatus(t, sqlDB, "legacy-admin-status", 0)
	if _, err := sqlDB.Exec(`INSERT INTO admin_users (username, password, real_name, email, auth_source, status) VALUES ('duplicate-email', 'hash', 'Duplicate Email', 'admin@example.com', 'local', 1)`); err == nil {
		t.Fatal("expected unique admin email index to reject duplicate")
	}

	// 校验最终 schema：列应处于演进后的最终状态
	assertNoColumn(t, sqlDB, "push_tasks", "title")
	assertNoColumn(t, sqlDB, "push_tasks", "content")
	assertNoColumn(t, sqlDB, "push_tasks", "provider_msg_id")
	assertNoColumn(t, sqlDB, "message_templates", "template_code")
	assertNoColumn(t, sqlDB, "message_templates", "message_type")
	assertHasColumn(t, sqlDB, "message_templates", "content_type")
	assertHasColumn(t, sqlDB, "provider_templates", "content_type")
	for _, table := range []string{"provider_templates", "provider_signatures"} {
		for _, column := range []string{"audit_status", "audit_reply", "synced_at", "remote_deleted"} {
			assertHasColumn(t, sqlDB, table, column)
		}
	}
	for _, column := range []string{"remote_id", "native_content", "variable_slots", "codec_version"} {
		assertNoColumn(t, sqlDB, "provider_templates", column)
	}

	assertHasColumn(t, sqlDB, "provider_templates", "content_version")
	assertHasColumn(t, sqlDB, "channel_template_bindings", "mapped_content_version")
	assertHasColumn(t, sqlDB, "provider_signatures", "remote_id")
	assertHasColumn(t, sqlDB, "push_logs", "provider_msg_id")
	assertHasColumn(t, sqlDB, "push_logs", "send_snapshot")
	assertHasColumn(t, sqlDB, "push_tasks", "provider_account_id")
	assertHasColumn(t, sqlDB, "push_tasks", "attachment_group_id")
	assertHasColumn(t, sqlDB, "email_attachments", "content")
	assertHasColumn(t, sqlDB, "email_attachments", "purged_at")
	assertHasIndex(t, sqlDB, "push_tasks", "idx_push_tasks_provider_account")
	assertHasIndex(t, sqlDB, "push_tasks", "idx_push_tasks_attachment_group")
	assertLastProvider(t, sqlDB, "provider-backfill", 2)
	assertHasColumn(t, sqlDB, "callback_logs", "type")
	assertHasColumn(t, sqlDB, "callback_logs", "mobile")
	assertHasColumn(t, sqlDB, "callback_logs", "content")
	assertNoColumn(t, sqlDB, "channel_template_bindings", "template_binding_id")
	assertCanonicalUTCTime(t, sqlDB, "applications", "created_at", "app_id", "legacy-app", "2026-08-04 10:00:00+00:00")
	assertCanonicalUTCTime(t, sqlDB, "applications", "updated_at", "app_id", "legacy-app", "2026-08-04 10:00:00+00:00")
	assertCanonicalUTCTime(t, sqlDB, "webhook_logs", "next_attempt_at", "task_id", "offset-webhook", "2026-08-04 10:00:00+00:00")
	assertCanonicalUTCTime(t, sqlDB, "app_quota_stats", "stat_date", "app_id", "legacy-app", "2026-08-04")
	due, err := dao.NewWebhookLogDAOWithDB(db).ListDue(context.Background(), time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC), 10)
	if err != nil {
		t.Fatalf("list due webhook after UTC migration: %v", err)
	}
	if len(due) != 1 || due[0].TaskID != "offset-webhook" {
		t.Fatalf("due webhooks = %+v, want offset-webhook", due)
	}
	for _, item := range append(append([]schemaColumn{}, utcInstantColumns...), businessDateColumns...) {
		assertHasColumn(t, sqlDB, item.table, item.column)
	}
	for _, column := range []string{
		"dedup_key",
		"signing_secret",
		"max_retries",
		"timeout_seconds",
		"next_attempt_at",
		"locked_until",
		"lease_token",
		"updated_at",
	} {
		assertHasColumn(t, sqlDB, "webhook_logs", column)
	}

	// 重复执行应为幂等（无新版本）
	if err := RunGooseMigrations(sqlDB, "sqlite", nil); err != nil {
		t.Fatalf("rerun migrations: %v", err)
	}
}

func assertCanonicalUTCTime(t *testing.T, db *sql.DB, table, column, keyColumn, keyValue, want string) {
	t.Helper()
	var got string
	query := "SELECT CAST(" + column + " AS TEXT) FROM " + table + " WHERE " + keyColumn + " = ?"
	if err := db.QueryRow(query, keyValue).Scan(&got); err != nil {
		t.Fatalf("query %s.%s: %v", table, column, err)
	}
	if got != want {
		t.Fatalf("%s.%s = %q, want %q", table, column, got, want)
	}
}

func TestSQLiteAdminEmailMigrationRejectsNormalizedDuplicates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/duplicate-email.db?_time_format=sqlite&_timezone=UTC"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	goose.SetBaseFS(migrationFS.FS())
	if err := goose.UpTo(sqlDB, migrationFS.DialectDir("sqlite"), 20260722000002); err != nil {
		t.Fatalf("run migrations before admin email uniqueness: %v", err)
	}

	seed := []string{
		`INSERT INTO admin_users (username, password, real_name, email, auth_source, status) VALUES ('first', 'hash', 'First', ' Duplicate@Example.com ', 'local', 1)`,
		`INSERT INTO admin_users (username, password, real_name, email, auth_source, status) VALUES ('second', 'hash', 'Second', 'duplicate@example.COM', 'local', 1)`,
	}
	for _, statement := range seed {
		if _, err := sqlDB.Exec(statement); err != nil {
			t.Fatalf("seed duplicate email: %v", err)
		}
	}

	err = goose.UpTo(sqlDB, migrationFS.DialectDir("sqlite"), 20260723000001)
	if err == nil {
		t.Fatal("expected migration to fail on normalized duplicate emails")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Fatalf("migration error = %v, want unique constraint failure", err)
	}

	var nullEmails int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM admin_users WHERE email IS NULL`).Scan(&nullEmails); err != nil {
		t.Fatalf("count null emails after failed migration: %v", err)
	}
	if nullEmails != 0 {
		t.Fatalf("failed migration cleared %d email records", nullEmails)
	}
	var userCount int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&userCount); err != nil {
		t.Fatalf("count users after failed migration: %v", err)
	}
	if userCount != 2 {
		t.Fatalf("failed migration changed account ownership: got %d users", userCount)
	}
}

func assertAdminEmail(t *testing.T, db *sql.DB, username, want string, wantValid bool) {
	t.Helper()
	var got sql.NullString
	if err := db.QueryRow("SELECT email FROM admin_users WHERE username = ?", username).Scan(&got); err != nil {
		t.Fatalf("query admin email for %s: %v", username, err)
	}
	if got.Valid != wantValid || got.String != want {
		t.Fatalf("admin email for %s = (%q, valid=%v), want (%q, valid=%v)", username, got.String, got.Valid, want, wantValid)
	}
}

func assertAdminStatus(t *testing.T, db *sql.DB, username string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("SELECT status FROM admin_users WHERE username = ?", username).Scan(&got); err != nil {
		t.Fatalf("query admin status for %s: %v", username, err)
	}
	if got != want {
		t.Errorf("admin status for %s = %d, want %d", username, got, want)
	}
}

func assertLastProvider(t *testing.T, db *sql.DB, taskID string, want int) {
	t.Helper()
	var got sql.NullInt64
	if err := db.QueryRow("SELECT provider_account_id FROM push_tasks WHERE task_id = ?", taskID).Scan(&got); err != nil {
		t.Fatalf("query last provider for %s: %v", taskID, err)
	}
	if !got.Valid || got.Int64 != int64(want) {
		t.Fatalf("last provider for %s = %v, want %d", taskID, got, want)
	}
}

func assertHasIndex(t *testing.T, db *sql.DB, table, index string) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND name = ?",
		table,
		index,
	).Scan(&count); err != nil {
		t.Fatalf("query index %s on %s: %v", index, table, err)
	}
	if count != 1 {
		t.Fatalf("index %s on %s count = %d, want 1", index, table, count)
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
