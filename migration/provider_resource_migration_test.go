package migration

import (
	"fmt"
	"strings"
	"testing"

	migrationFS "cnb.cool/mliev/push/message-push/migrations"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func TestProviderResourceMigrationDialectsAndSafeRollback(t *testing.T) {
	const name = "20260909000001_provider_resource_management.sql"
	for _, dialect := range []string{"mysql", "pgsql", "sqlite"} {
		data, err := migrationFS.FS().ReadFile(dialect + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range []string{"-- +goose Up", "-- +goose Down", "audit_status", "variable_slots", "idx_provider_templates_remote_resource", "idx_provider_signatures_remote_resource"} {
			if !strings.Contains(string(data), fragment) {
				t.Fatalf("%s missing %s", dialect, fragment)
			}
		}
	}
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/resource-rollback.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	goose.SetBaseFS(migrationFS.FS())
	if err = goose.Up(sqlDB, "sqlite"); err != nil {
		t.Fatal(err)
	}
	for id, audit := range []string{"NULL", "1", "2", "3"} {
		if _, err = sqlDB.Exec(fmt.Sprintf(`INSERT INTO provider_templates (provider_id, template_code, template_name, status, audit_status) VALUES (1, '%d', 'keep me', 1, %s)`, id, audit)); err != nil {
			t.Fatal(err)
		}
	}
	if err = goose.Down(sqlDB, "sqlite"); err != nil {
		t.Fatal(err)
	}
	assertNoColumn(t, sqlDB, "provider_templates", "audit_status")
	assertNoColumn(t, sqlDB, "provider_signatures", "remote_id")
	for code, want := range []int{1, 0, 1, 0} {
		var status int
		var name string
		if err = sqlDB.QueryRow("SELECT status, template_name FROM provider_templates WHERE template_code = ?", fmt.Sprint(code)).Scan(&status, &name); err != nil {
			t.Fatal(err)
		}
		if status != want || name != "keep me" {
			t.Fatalf("rollback changed record %d: status=%d name=%s", code, status, name)
		}
	}
	if err = goose.Up(sqlDB, "sqlite"); err != nil {
		t.Fatal(err)
	}
	assertHasColumn(t, sqlDB, "provider_templates", "audit_status")
}
