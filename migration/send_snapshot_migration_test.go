package migration

import (
	"database/sql"
	"strings"
	"testing"

	migrationFS "cnb.cool/mliev/push/message-push/migrations"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func TestSendSnapshotMigrationPreservesLegacyLogsAndRollsBack(t *testing.T) {
	const name = "20260910000001_add_send_snapshot.sql"
	for dialect, columnType := range map[string]string{"mysql": "JSON", "pgsql": "JSONB", "sqlite": "TEXT"} {
		data, err := migrationFS.FS().ReadFile(dialect + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range []string{"-- +goose Up", "ADD COLUMN send_snapshot " + columnType, "-- +goose Down", "DROP COLUMN send_snapshot"} {
			if !strings.Contains(string(data), fragment) {
				t.Fatalf("%s missing %q", dialect, fragment)
			}
		}
	}
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/snapshot-migration.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	goose.SetBaseFS(migrationFS.FS())
	if err := goose.UpTo(sqlDB, "sqlite", 20260909000001); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`INSERT INTO push_logs (task_id, app_id, provider_account_id, request_data, status) VALUES ('legacy', 'app', 1, '{"templateid":"16021"}', 'success')`); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(sqlDB, "sqlite", 20260910000001); err != nil {
		t.Fatal(err)
	}
	var snapshot sql.NullString
	if err := sqlDB.QueryRow(`SELECT send_snapshot FROM push_logs WHERE task_id = 'legacy'`).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Valid {
		t.Fatalf("legacy snapshot was fabricated: %v", snapshot)
	}
	if _, err := sqlDB.Exec(`UPDATE push_logs SET send_snapshot = '{"version":1,"content":"正文"}' WHERE task_id = 'legacy'`); err != nil {
		t.Fatal(err)
	}
	if err := goose.Down(sqlDB, "sqlite"); err != nil {
		t.Fatal(err)
	}
	assertNoColumn(t, sqlDB, "push_logs", "send_snapshot")
	var request string
	if err := sqlDB.QueryRow(`SELECT request_data FROM push_logs WHERE task_id = 'legacy'`).Scan(&request); err != nil {
		t.Fatal(err)
	}
	if request != `{"templateid":"16021"}` {
		t.Fatalf("rollback changed request: %s", request)
	}
	if err := goose.UpTo(sqlDB, "sqlite", 20260910000001); err != nil {
		t.Fatal(err)
	}
	assertHasColumn(t, sqlDB, "push_logs", "send_snapshot")
}
