package migration

import (
	"context"
	"io"
	"io/fs"
	"log"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/migrations"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func TestTencentSMSMigrationPreservesExistingBindings(t *testing.T) {
	for _, dialect := range []string{"mysql", "pgsql", "sqlite"} {
		data, err := migrations.FS().ReadFile(dialect + "/20260910000003_tencent_sms_management.sql")
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"provider_sms_events", "provider_sms_pull_runs", "provider_sms_polling", "uk_provider_sms_events_key", "uk_provider_sms_polling_owner", "provider_metadata", "sms_event_key", "-- +goose Down"} {
			if !strings.Contains(string(data), field) {
				t.Fatalf("%s missing %s", dialect, field)
			}
		}
	}
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/sms-migration.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	sub, err := fs.Sub(migrations.FS(), "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, sqlDB, sub, goose.WithDisableGlobalRegistry(true), goose.WithLogger(log.New(io.Discard, "", 0)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(context.Background(), 20260910000002); err != nil {
		t.Fatal(err)
	}
	if _, err = sqlDB.Exec(`INSERT INTO provider_templates(id,provider_id,template_code,template_name,template_content,content_version,status) VALUES(1,1,'101','keep','验证码{1}',7,1)`); err != nil {
		t.Fatal(err)
	}
	if _, err = sqlDB.Exec(`INSERT INTO channel_template_bindings(id,channel_id,provider_template_id,provider_id,param_mapping,mapped_content_version,status) VALUES(1,1,1,1,'[{"provider_var":"1","system_var":"code","type":"mapping"}]',7,1)`); err != nil {
		t.Fatal(err)
	}
	assertBinding := func() {
		t.Helper()
		var mapping string
		var version, status int
		if err := sqlDB.QueryRow(`SELECT param_mapping,mapped_content_version,status FROM channel_template_bindings WHERE id=1`).Scan(&mapping, &version, &status); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(mapping, "code") || version != 7 || status != 1 {
			t.Fatalf("existing binding changed: %s %d %d", mapping, version, status)
		}
	}
	if _, err = provider.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertBinding()
	if _, err = sqlDB.Exec(`INSERT INTO provider_sms_polling(provider_account_id,kind,sdk_app_id,owner_key,updated_at) VALUES(1,'reports','140001','140001:reports','2026-09-10')`); err != nil {
		t.Fatal(err)
	}
	if _, err = sqlDB.Exec(`INSERT INTO provider_sms_polling(provider_account_id,kind,sdk_app_id,owner_key,updated_at) VALUES(2,'reports','140001','140001:reports','2026-09-10')`); err == nil {
		t.Fatal("duplicate active queue owner accepted")
	}
	if _, err = provider.DownTo(context.Background(), 20260910000002); err != nil {
		t.Fatal(err)
	}
	assertBinding()
}
