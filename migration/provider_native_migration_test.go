package migration

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	migrationFS "cnb.cool/mliev/push/message-push/migrations"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func nativeMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/native.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	goose.SetBaseFS(migrationFS.FS())
	if err = goose.UpTo(sqlDB, "sqlite", 20260910000001); err != nil {
		t.Fatal(err)
	}
	return sqlDB
}

func TestNativeTemplateMigrationResetIdentityAndRollback(t *testing.T) {
	db := nativeMigrationDB(t)
	statements := []string{
		`INSERT INTO provider_accounts(id,account_code,account_name,provider_code,provider_type,config,status) VALUES (1,'sms','短信','zrwinfo_sms','sms','{}',1),(2,'mail','邮件','smtp','email','{}',1)`,
		`INSERT INTO channels(id,name,type,status) VALUES(1,'短信','sms',1),(2,'邮件','email',1)`,
		`INSERT INTO provider_templates(id,provider_id,template_code,template_name,template_content,native_content,remote_id,variables,status,remark) VALUES(1,1,'11','本地名称','主机{alias}','主机{1}','different-id','["alias"]',1,'保留'),(2,1,'12','手工','规则{rule}','','','["rule"]',1,''),(3,2,'mail','邮件','你好{name}','','','["name"]',1,'')`,
		`INSERT INTO channel_template_bindings(id,channel_id,provider_template_id,provider_id,param_mapping,status,is_active,deleted_at) VALUES(1,1,1,1,'[{"type":"fixed","provider_var":"alias","value":"old"}]',1,1,NULL),(2,1,2,1,'old',0,0,NULL),(3,1,1,1,'old',1,1,'2026-01-01'),(4,2,3,2,'[{"type":"mapping","provider_var":"name","system_var":"name"}]',1,1,NULL)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := RunGooseMigrations(db, "sqlite", nil); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"remote_id", "native_content", "variable_slots", "codec_version"} {
		assertNoColumn(t, db, "provider_templates", column)
	}
	assertHasColumn(t, db, "provider_signatures", "remote_id")
	var content, name, remark, code string
	var version int
	if err := db.QueryRow("SELECT template_content,template_name,remark,template_code,content_version FROM provider_templates WHERE id=1").Scan(&content, &name, &remark, &code, &version); err != nil {
		t.Fatal(err)
	}
	if content != "主机{1}" || name != "本地名称" || remark != "保留" || code != "11" || version != 1 {
		t.Fatalf("wrong migration: %q %q %q %q %d", content, name, remark, code, version)
	}
	for id := 1; id <= 4; id++ {
		var mapping string
		var mapped int
		if err := db.QueryRow("SELECT param_mapping,mapped_content_version FROM channel_template_bindings WHERE id=?", id).Scan(&mapping, &mapped); err != nil {
			t.Fatal(err)
		}
		if id < 4 && (mapping != "[]" || mapped != 0) {
			t.Fatalf("SMS binding %d was not reset: %q/%d", id, mapping, mapped)
		}
		if id == 4 && !strings.Contains(mapping, "system_var") {
			t.Fatal("email mapping changed")
		}
	}
	if _, err := db.Exec(`INSERT INTO provider_templates(provider_id,template_code,template_name) VALUES(1,'11','duplicate')`); err == nil {
		t.Fatal("duplicate live code accepted")
	}
	for i := 0; i < 2; i++ {
		if _, err := db.Exec(`INSERT INTO provider_templates(provider_id,template_code,template_name,deleted_at) VALUES(1,'11','deleted','2026-01-01')`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO provider_templates(provider_id,template_code,template_name) VALUES(2,'11','other account')`); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(db, "sqlite", 20260910000001); err != nil {
		t.Fatal(err)
	}
	for id := 1; id <= 3; id++ {
		var status int
		if err := db.QueryRow("SELECT status FROM channel_template_bindings WHERE id=?", id).Scan(&status); err != nil || status != 0 {
			t.Fatalf("rollback SMS %d: %d %v", id, status, err)
		}
	}
	assertHasColumn(t, db, "provider_templates", "remote_id")
}

func TestNativeTemplateMigrationReportsDuplicatesBeforeChanges(t *testing.T) {
	db := nativeMigrationDB(t)
	for i := 0; i < 2; i++ {
		if _, err := db.Exec(`INSERT INTO provider_templates(provider_id,template_code,template_name) VALUES(9,'same-code','keep')`); err != nil {
			t.Fatal(err)
		}
	}
	if err := RunGooseMigrations(db, "sqlite", nil); err == nil || !strings.Contains(err.Error(), "账号 9 / 模板 same-code") {
		t.Fatalf("conflict: %v", err)
	}
	assertNoColumn(t, db, "provider_templates", "content_version")
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM provider_templates").Scan(&count); err != nil || count != 2 {
		t.Fatalf("records changed: %d %v", count, err)
	}
}

func TestNativeTemplateMigrationDialectContract(t *testing.T) {
	for _, dialect := range []string{"mysql", "pgsql", "sqlite"} {
		raw, err := migrationFS.FS().ReadFile(fmt.Sprintf("%s/20260910000002_provider_native_templates.sql", dialect))
		if err != nil {
			t.Fatal(err)
		}
		for _, fragment := range []string{"-- +goose Up", "-- +goose Down", "mapped_content_version", "content_version", "uk_provider_templates_account_code_live", "DROP COLUMN variable_slots", "DROP COLUMN remote_id", "SET status = 0"} {
			if !strings.Contains(string(raw), fragment) {
				t.Fatalf("%s missing %s", dialect, fragment)
			}
		}
	}
}
