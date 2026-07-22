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
	users      []*model.AdminUser
	nextID     uint
	rejectBind bool
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

func (f *fakeAdminUserStore) BindOidcSub(id uint, sub string) (bool, error) {
	if f.rejectBind {
		return false, nil
	}
	for _, u := range f.users {
		if u.ID == id {
			u.OidcSub = &sub
			return true, nil
		}
	}
	return false, gorm.ErrRecordNotFound
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

func TestProvisionOIDCUser_NormalizesEmailBeforeMatching(t *testing.T) {
	store := &fakeAdminUserStore{
		users: []*model.AdminUser{
			{ID: 1, Username: "normalized", Email: strPtr("normalized@example.com"), Status: 1},
		},
		nextID: 1,
	}

	user, err := provisionOIDCUser(store, "sub-normalized", "  Normalized@Example.COM ", "", "")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if user.ID != 1 {
		t.Fatalf("expected existing user 1, got %d", user.ID)
	}
	if store.users[0].OidcSub == nil || *store.users[0].OidcSub != "sub-normalized" {
		t.Fatalf("expected normalized email match to bind subject, got %v", store.users[0].OidcSub)
	}
}

func TestProvisionOIDCUser_RejectsEmailBoundToDifferentSubject(t *testing.T) {
	store := &fakeAdminUserStore{
		users: []*model.AdminUser{
			{
				ID:       1,
				Username: "bound",
				Email:    strPtr("bound@example.com"),
				OidcSub:  strPtr("existing-sub"),
				Status:   1,
			},
		},
		nextID: 1,
	}

	_, err := provisionOIDCUser(store, "different-sub", "bound@example.com", "", "")
	if !errors.Is(err, domain.ErrOIDCIdentityConflict) {
		t.Fatalf("expected ErrOIDCIdentityConflict, got %v", err)
	}
	if got := *store.users[0].OidcSub; got != "existing-sub" {
		t.Fatalf("existing subject was overwritten: got %q", got)
	}
	if len(store.users) != 1 {
		t.Fatalf("conflict must not create a user, got %d users", len(store.users))
	}
}

func TestProvisionOIDCUser_RejectsConcurrentBindingConflict(t *testing.T) {
	store := &fakeAdminUserStore{
		users: []*model.AdminUser{
			{ID: 1, Username: "race", Email: strPtr("race@example.com"), Status: 1},
		},
		nextID:     1,
		rejectBind: true,
	}

	_, err := provisionOIDCUser(store, "losing-sub", "race@example.com", "", "")
	if !errors.Is(err, domain.ErrOIDCIdentityConflict) {
		t.Fatalf("conditional bind miss must map to ErrOIDCIdentityConflict, got %v", err)
	}
	if store.users[0].OidcSub != nil {
		t.Fatalf("losing callback overwrote subject: %v", store.users[0].OidcSub)
	}
}

func TestProvisionOIDCUser_RejectsInvalidEmailClaim(t *testing.T) {
	store := &fakeAdminUserStore{}

	if _, err := provisionOIDCUser(store, "sub-invalid-email", "not-an-email", "", ""); err == nil {
		t.Fatal("expected invalid email claim to be rejected")
	}
	if len(store.users) != 0 {
		t.Fatalf("invalid claim must not create a user, got %d users", len(store.users))
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
