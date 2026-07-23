package main

import (
	"context"
	"path/filepath"
	"testing"

	appHelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestResetDemoCreatesValidDeterministicDataset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.sqlite")
	for attempt := 0; attempt < 2; attempt++ {
		summary, err := resetDemo(context.Background(), resetOptions{DBPath: path})
		if err != nil {
			t.Fatalf("reset attempt %d: %v", attempt+1, err)
		}
		if summary.Tasks != 93 || summary.Applications != 3 || summary.Channels != 4 || summary.UpstreamMessages != 3 {
			t.Fatalf("unexpected summary after attempt %d: %+v", attempt+1, summary)
		}
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	var admin model.AdminUser
	if err := db.Where("username = ?", "demo-admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte("demo-pass-2026")); err != nil {
		t.Fatalf("demo password hash does not match: %v", err)
	}
	var app model.Application
	if err := db.Where("app_id = ?", "demo_shop").First(&app).Error; err != nil {
		t.Fatal(err)
	}
	secret, err := appHelper.DecryptAppSecret(app.AppSecret)
	if err != nil {
		t.Fatal(err)
	}
	if secret != "demo-shop-secret-2026" {
		t.Fatalf("unexpected decrypted app secret: %q", secret)
	}
	var batchLinked int64
	if err := db.Table("push_tasks").Where("batch_id <> ''").Count(&batchLinked).Error; err != nil {
		t.Fatal(err)
	}
	if batchLinked != 21 {
		t.Fatalf("batch-linked task count = %d, want 21", batchLinked)
	}
	assertCount(t, db, "applications", "status = 0", 1)
	assertCount(t, db, "admin_users", "status = 0", 1)
	assertCount(t, db, "provider_accounts", "status = 0", 1)
	assertCount(t, db, "message_templates", "status = 0", 1)
	assertCount(t, db, "channels", "status = 0", 1)
	assertCount(t, db, "channel_template_bindings", "is_active = 0", 1)
	assertCount(t, db, "failure_rules", "status = 0", 1)

	var processing model.PushBatchTask
	if err := db.Where("batch_id = ?", "b0000000-0000-4000-8000-000000000002").First(&processing).Error; err != nil {
		t.Fatal(err)
	}
	if processing.TotalCount != 9 || processing.SuccessCount != 6 || processing.FailedCount != 1 || processing.PendingCount != 2 {
		t.Fatalf("unexpected processing batch summary: %+v", processing)
	}
	var currentVersion int64
	if err := db.Table("goose_db_version").Select("MAX(version_id)").Scan(&currentVersion).Error; err != nil {
		t.Fatal(err)
	}
	if currentVersion != 20260723000001 {
		t.Fatalf("migration version = %d", currentVersion)
	}
}

func assertCount(t *testing.T, db *gorm.DB, table, where string, want int64) {
	t.Helper()
	var got int64
	if err := db.Table(table).Where(where).Count(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s where %q count = %d, want %d", table, where, got, want)
	}
}

func TestValidateDBPathRejectsBroadOrUnknownTargets(t *testing.T) {
	invalid := []string{"", ".", "demo.txt"}
	for _, value := range invalid {
		if _, err := validateDBPath(value); err == nil {
			t.Errorf("validateDBPath(%q) unexpectedly succeeded", value)
		}
	}
}

func TestRunRejectsNonDedicatedRedisDB(t *testing.T) {
	err := run(context.Background(), []string{"reset", "--db", filepath.Join(t.TempDir(), "demo.sqlite"), "--redis-db", "0"})
	if err == nil {
		t.Fatal("run accepted Redis DB 0")
	}
}
