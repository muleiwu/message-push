package dao

import (
	"path/filepath"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestBindOidcSubUsesConditionalUpdate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "admin-user-dao.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AdminUser{}); err != nil {
		t.Fatalf("migrate admin user: %v", err)
	}
	dao := &AdminUserDAO{db: db}

	unbound := &model.AdminUser{Username: "unbound", Password: "hash", RealName: "Unbound", AuthSource: "local", Status: 1}
	if err := db.Create(unbound).Error; err != nil {
		t.Fatalf("create unbound user: %v", err)
	}
	bound, err := dao.BindOidcSub(unbound.ID, "first-sub")
	if err != nil || !bound {
		t.Fatalf("bind unbound user = (%v, %v), want (true, nil)", bound, err)
	}

	otherSub := "other-sub"
	alreadyBound := &model.AdminUser{
		Username: "bound", Password: "hash", RealName: "Bound", AuthSource: "local", Status: 1, OidcSub: &otherSub,
	}
	if err := db.Create(alreadyBound).Error; err != nil {
		t.Fatalf("create bound user: %v", err)
	}
	bound, err = dao.BindOidcSub(alreadyBound.ID, "different-sub")
	if err != nil {
		t.Fatalf("conflicting bind error = %v", err)
	}
	if bound {
		t.Fatal("conflicting bind = true, want false")
	}

	var persisted model.AdminUser
	if err := db.First(&persisted, alreadyBound.ID).Error; err != nil {
		t.Fatalf("reload bound user: %v", err)
	}
	if persisted.OidcSub == nil || *persisted.OidcSub != otherSub {
		t.Fatalf("conditional update overwrote subject: %v", persisted.OidcSub)
	}
}

func TestAdminUserEmailExistsExcludesCurrentAndIncludesSoftDeleted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "admin-user-email.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AdminUser{}); err != nil {
		t.Fatalf("migrate admin user: %v", err)
	}
	dao := &AdminUserDAO{db: db}
	email := "admin@example.com"
	user := &model.AdminUser{
		Username: "email-user", Password: "hash", RealName: "Email User", Email: &email, AuthSource: "local", Status: 1,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create admin user: %v", err)
	}

	exists, err := dao.EmailExists(email, 0)
	if err != nil || !exists {
		t.Fatalf("EmailExists(email, 0) = (%v, %v), want (true, nil)", exists, err)
	}
	exists, err = dao.EmailExists(email, user.ID)
	if err != nil || exists {
		t.Fatalf("EmailExists(email, own id) = (%v, %v), want (false, nil)", exists, err)
	}

	if err := db.Delete(user).Error; err != nil {
		t.Fatalf("soft delete admin user: %v", err)
	}
	exists, err = dao.EmailExists(email, 0)
	if err != nil || !exists {
		t.Fatalf("EmailExists(email, 0) after soft delete = (%v, %v), want (true, nil)", exists, err)
	}
}

func TestAdminUserUpdatePreservesConcurrentIdentityFieldsAndWritesDisabledStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "admin-user-update.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.AdminUser{}); err != nil {
		t.Fatalf("migrate admin user: %v", err)
	}
	dao := &AdminUserDAO{db: db}
	oldEmail := "old@example.com"
	user := &model.AdminUser{
		Username: "update-user", Password: "old-hash", RealName: "Old Name", Email: &oldEmail, AuthSource: "local", Status: 1,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	stale, err := dao.GetByID(user.ID)
	if err != nil {
		t.Fatalf("get stale admin user: %v", err)
	}
	if err := db.Model(&model.AdminUser{}).Where("id = ?", user.ID).Updates(map[string]any{
		"password": "new-hash",
		"oidc_sub": "new-subject",
	}).Error; err != nil {
		t.Fatalf("concurrently update identity fields: %v", err)
	}

	newEmail := "new@example.com"
	stale.RealName = "New Name"
	stale.Email = &newEmail
	stale.Status = 0
	if err := dao.Update(stale); err != nil {
		t.Fatalf("update mutable admin fields: %v", err)
	}

	var persisted model.AdminUser
	if err := db.First(&persisted, user.ID).Error; err != nil {
		t.Fatalf("reload admin user: %v", err)
	}
	if persisted.Password != "new-hash" {
		t.Fatalf("password = %q, want concurrent value", persisted.Password)
	}
	if persisted.OidcSub == nil || *persisted.OidcSub != "new-subject" {
		t.Fatalf("oidc_sub = %v, want concurrent binding", persisted.OidcSub)
	}
	if persisted.RealName != "New Name" || persisted.Email == nil || *persisted.Email != newEmail {
		t.Fatalf("mutable fields = (%q, %v), want updated", persisted.RealName, persisted.Email)
	}
	if persisted.Status != 0 {
		t.Fatalf("status = %d, want explicit disabled value", persisted.Status)
	}
}
