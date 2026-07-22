package infrastructure

import (
	"path/filepath"
	"testing"

	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestCreateInitialDataPersistsNormalizedEmail(t *testing.T) {
	db := newInstallTestDB(t)
	service := NewInstallService(db)

	err := service.CreateInitialData(dto.AdminAccountInfo{
		Username: "installer",
		Password: "install-password",
		Email:    "  Installer@Example.COM ",
		RealName: "Initial Admin",
	})
	if err != nil {
		t.Fatalf("CreateInitialData() error = %v", err)
	}

	var user model.AdminUser
	if err := db.Where("username = ?", "installer").First(&user).Error; err != nil {
		t.Fatalf("query initial admin: %v", err)
	}
	if user.Email == nil || *user.Email != "installer@example.com" {
		t.Fatalf("stored email = %v, want installer@example.com", user.Email)
	}
	if user.AuthSource != "local" {
		t.Fatalf("auth source = %q, want local", user.AuthSource)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("install-password")); err != nil {
		t.Fatalf("stored password is not the expected bcrypt hash: %v", err)
	}
}

func TestCreateInitialDataRejectsMissingOrInvalidEmail(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{name: "missing", email: "  "},
		{name: "invalid", email: "not-an-email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newInstallTestDB(t)
			service := NewInstallService(db)
			err := service.CreateInitialData(dto.AdminAccountInfo{
				Username: "installer",
				Password: "install-password",
				Email:    tt.email,
				RealName: "Initial Admin",
			})
			if err == nil {
				t.Fatalf("CreateInitialData() error = nil for email %q", tt.email)
			}

			var count int64
			if err := db.Model(&model.AdminUser{}).Count(&count).Error; err != nil {
				t.Fatalf("count admins: %v", err)
			}
			if count != 0 {
				t.Fatalf("invalid input created %d admins", count)
			}
		})
	}
}

func newInstallTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "install.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AdminUser{}); err != nil {
		t.Fatalf("migrate admin user: %v", err)
	}
	return db
}
