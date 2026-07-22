package server

import (
	"path/filepath"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type fakeStringEnv map[string]string

func (f fakeStringEnv) GetString(key, defaultValue string) string {
	if value, ok := f[key]; ok {
		return value
	}
	return defaultValue
}

func TestReadBootstrapAdminSettingsReadsOptionalEmail(t *testing.T) {
	settings := readBootstrapAdminSettings(fakeStringEnv{
		"admin.username":  "bootstrap",
		"admin.password":  "password",
		"admin.real_name": "Bootstrap Admin",
		"admin.email":     "Bootstrap@Example.COM",
	})

	if settings.email != "Bootstrap@Example.COM" {
		t.Fatalf("email = %q, want value from admin.email (ADMIN_EMAIL)", settings.email)
	}
	if settings.username != "bootstrap" || settings.password != "password" || settings.realName != "Bootstrap Admin" {
		t.Fatalf("unexpected settings: %+v", settings)
	}
}

func TestReadBootstrapAdminSettingsDefaultsRealNameAndEmail(t *testing.T) {
	settings := readBootstrapAdminSettings(fakeStringEnv{
		"admin.username": "bootstrap",
		"admin.password": "password",
	})

	if settings.realName != "bootstrap" {
		t.Fatalf("real name = %q, want username fallback", settings.realName)
	}
	if settings.email != "" {
		t.Fatalf("email = %q, want empty optional value", settings.email)
	}
}

func TestCreateBootstrapAdminPersistsOptionalNormalizedEmail(t *testing.T) {
	db := newBootstrapTestDB(t)

	created, err := createBootstrapAdmin(db, "bootstrap", "bootstrap-password", "Bootstrap Admin", "  Bootstrap@Example.COM ")
	if err != nil {
		t.Fatalf("createBootstrapAdmin() error = %v", err)
	}
	if !created {
		t.Fatal("createBootstrapAdmin() created = false, want true")
	}

	var user model.AdminUser
	if err := db.Where("username = ?", "bootstrap").First(&user).Error; err != nil {
		t.Fatalf("query bootstrap admin: %v", err)
	}
	if user.Email == nil || *user.Email != "bootstrap@example.com" {
		t.Fatalf("stored email = %v, want bootstrap@example.com", user.Email)
	}
}

func TestCreateBootstrapAdminAllowsMissingEmail(t *testing.T) {
	db := newBootstrapTestDB(t)

	created, err := createBootstrapAdmin(db, "legacy", "bootstrap-password", "Legacy Bootstrap", "")
	if err != nil {
		t.Fatalf("createBootstrapAdmin() error = %v", err)
	}
	if !created {
		t.Fatal("createBootstrapAdmin() created = false, want true")
	}

	var user model.AdminUser
	if err := db.Where("username = ?", "legacy").First(&user).Error; err != nil {
		t.Fatalf("query bootstrap admin: %v", err)
	}
	if user.Email != nil {
		t.Fatalf("stored email = %v, want nil", user.Email)
	}
}

func TestCreateBootstrapAdminRejectsInvalidEmail(t *testing.T) {
	db := newBootstrapTestDB(t)

	created, err := createBootstrapAdmin(db, "invalid", "bootstrap-password", "Invalid Bootstrap", "not-an-email")
	if err == nil {
		t.Fatal("createBootstrapAdmin() error = nil, want invalid email error")
	}
	if created {
		t.Fatal("createBootstrapAdmin() created = true for invalid email")
	}

	var count int64
	if err := db.Model(&model.AdminUser{}).Count(&count).Error; err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if count != 0 {
		t.Fatalf("invalid email created %d admins", count)
	}
}

func TestCreateBootstrapAdminIsIdempotent(t *testing.T) {
	db := newBootstrapTestDB(t)
	existing := &model.AdminUser{
		Username:   "existing",
		Password:   "already-hashed",
		RealName:   "Existing Admin",
		AuthSource: "local",
		Status:     1,
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("create existing admin: %v", err)
	}

	created, err := createBootstrapAdmin(db, "new", "password", "New Admin", "bad-email-is-ignored-on-skip")
	if err != nil {
		t.Fatalf("createBootstrapAdmin() error = %v", err)
	}
	if created {
		t.Fatal("createBootstrapAdmin() created = true with existing admin")
	}

	var count int64
	if err := db.Model(&model.AdminUser{}).Count(&count).Error; err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if count != 1 {
		t.Fatalf("admin count = %d, want 1", count)
	}
}

func newBootstrapTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "bootstrap.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AdminUser{}); err != nil {
		t.Fatalf("migrate admin user: %v", err)
	}
	return db
}
