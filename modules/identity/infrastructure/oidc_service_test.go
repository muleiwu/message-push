package infrastructure

import (
	"errors"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/identity/domain"
	"gorm.io/gorm"
)

// fakeAdminUserStore 内存版 adminUserStore，用于 JIT 逻辑单测。
type fakeAdminUserStore struct {
	users  []*model.AdminUser
	nextID uint
}

func (f *fakeAdminUserStore) GetByOidcSub(sub string) (*model.AdminUser, error) {
	for _, u := range f.users {
		if u.OidcSub != nil && *u.OidcSub == sub {
			copied := *u
			return &copied, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminUserStore) GetByEmail(email string) (*model.AdminUser, error) {
	for _, u := range f.users {
		if u.Email != nil && *u.Email == email {
			copied := *u
			return &copied, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeAdminUserStore) BindOidcSub(id uint, sub string) error {
	for _, u := range f.users {
		if u.ID == id {
			u.OidcSub = &sub
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (f *fakeAdminUserStore) UsernameExists(username string) bool {
	for _, u := range f.users {
		if u.Username == username {
			return true
		}
	}
	return false
}

func (f *fakeAdminUserStore) Create(user *model.AdminUser) error {
	f.nextID++
	user.ID = f.nextID
	f.users = append(f.users, user)
	return nil
}

func strPtr(s string) *string { return &s }

func TestProvisionOIDCUser_MatchByOidcSub(t *testing.T) {
	store := &fakeAdminUserStore{
		users: []*model.AdminUser{
			{ID: 1, Username: "alice", OidcSub: strPtr("sub-1"), Status: 1},
		},
		nextID: 1,
	}

	user, err := provisionOIDCUser(store, "sub-1", "other@example.com", "Alice", "alice2")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if user.ID != 1 || user.Username != "alice" {
		t.Fatalf("expected existing user alice(1), got %s(%d)", user.Username, user.ID)
	}
	if len(store.users) != 1 {
		t.Fatalf("no new user should be created, got %d users", len(store.users))
	}
}

func TestProvisionOIDCUser_MatchByEmailBindsSub(t *testing.T) {
	store := &fakeAdminUserStore{
		users: []*model.AdminUser{
			{ID: 1, Username: "bob", Email: strPtr("bob@example.com"), Status: 1},
		},
		nextID: 1,
	}

	user, err := provisionOIDCUser(store, "sub-2", "bob@example.com", "Bob", "")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if user.ID != 1 {
		t.Fatalf("expected existing user bob(1), got %d", user.ID)
	}
	if store.users[0].OidcSub == nil || *store.users[0].OidcSub != "sub-2" {
		t.Fatalf("expected oidc_sub bound to sub-2, got %v", store.users[0].OidcSub)
	}
}

func TestProvisionOIDCUser_CreatesNewUser(t *testing.T) {
	store := &fakeAdminUserStore{}

	user, err := provisionOIDCUser(store, "sub-3", "carol@example.com", "Carol", "carol")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if user.Username != "carol" {
		t.Fatalf("expected username carol, got %s", user.Username)
	}
	if user.OidcSub == nil || *user.OidcSub != "sub-3" {
		t.Fatalf("expected oidc_sub sub-3, got %v", user.OidcSub)
	}
	if user.AuthSource != "oidc" {
		t.Fatalf("expected auth_source oidc, got %s", user.AuthSource)
	}
	if user.Status != 1 {
		t.Fatalf("expected status 1, got %d", user.Status)
	}
	if user.Email == nil || *user.Email != "carol@example.com" {
		t.Fatalf("expected email carol@example.com, got %v", user.Email)
	}
	if user.Password == "" {
		t.Fatal("expected random bcrypt password, got empty")
	}
}

func TestProvisionOIDCUser_UsernameCollisionAddsSuffix(t *testing.T) {
	store := &fakeAdminUserStore{
		users: []*model.AdminUser{
			{ID: 1, Username: "dave", Status: 1},
			{ID: 2, Username: "dave_1", Status: 1},
		},
		nextID: 2,
	}

	user, err := provisionOIDCUser(store, "sub-4", "dave@example.com", "Dave", "dave")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if user.Username != "dave_2" {
		t.Fatalf("expected username dave_2, got %s", user.Username)
	}
}

func TestProvisionOIDCUser_UsernameFallsBackToEmailLocalPart(t *testing.T) {
	store := &fakeAdminUserStore{}

	user, err := provisionOIDCUser(store, "sub-5", "erin@example.com", "", "")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if user.Username != "erin" {
		t.Fatalf("expected username erin, got %s", user.Username)
	}
	if user.RealName != "erin" {
		t.Fatalf("expected real name fallback erin, got %s", user.RealName)
	}
}

func TestProvisionOIDCUser_UsernameFallsBackToSub(t *testing.T) {
	store := &fakeAdminUserStore{}

	user, err := provisionOIDCUser(store, "1234567890abcdef", "", "", "")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if user.Username != "oidc_12345678" {
		t.Fatalf("expected username oidc_12345678, got %s", user.Username)
	}
	if user.Email != nil {
		t.Fatalf("expected nil email, got %v", user.Email)
	}
}

func TestProvisionOIDCUser_DisabledUserRejected(t *testing.T) {
	store := &fakeAdminUserStore{
		users: []*model.AdminUser{
			{ID: 1, Username: "frank", OidcSub: strPtr("sub-6"), Status: 0},
		},
		nextID: 1,
	}

	if _, err := provisionOIDCUser(store, "sub-6", "", "", ""); !errors.Is(err, domain.ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
}

func TestProvisionOIDCUser_DisabledUserByEmailRejectedWithoutBinding(t *testing.T) {
	store := &fakeAdminUserStore{
		users: []*model.AdminUser{
			{ID: 1, Username: "grace", Email: strPtr("grace@example.com"), Status: 0},
		},
		nextID: 1,
	}

	if _, err := provisionOIDCUser(store, "sub-7", "grace@example.com", "", ""); !errors.Is(err, domain.ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
	if store.users[0].OidcSub != nil {
		t.Fatal("disabled user must not get oidc_sub bound")
	}
}

func TestProvisionOIDCUser_EmptySubRejected(t *testing.T) {
	store := &fakeAdminUserStore{}
	if _, err := provisionOIDCUser(store, "", "x@example.com", "", ""); err == nil {
		t.Fatal("expected error for empty subject")
	}
}
