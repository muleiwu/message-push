package infrastructure

import (
	"errors"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/identity/domain"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type fakeAdminCRUDStore struct {
	users                   map[uint]*model.AdminUser
	nextID                  uint
	createErr               error
	updateErr               error
	emailExistsErr          error
	forceEmailConflict      bool
	conflictAfterFirstCheck bool
	emailCheckCount         int
	lastEmailExcludeID      uint
	lastListStatus          *int8
	lastPasswordUserID      uint
	lastHashedPassword      string
}

func newFakeAdminCRUDStore(users ...*model.AdminUser) *fakeAdminCRUDStore {
	store := &fakeAdminCRUDStore{users: make(map[uint]*model.AdminUser)}
	for _, user := range users {
		copied := *user
		store.users[user.ID] = &copied
		if user.ID > store.nextID {
			store.nextID = user.ID
		}
	}
	return store
}

func (f *fakeAdminCRUDStore) Create(user *model.AdminUser) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.nextID++
	user.ID = f.nextID
	copied := *user
	f.users[user.ID] = &copied
	return nil
}

func (f *fakeAdminCRUDStore) Delete(id uint) error {
	delete(f.users, id)
	return nil
}

func (f *fakeAdminCRUDStore) EmailExists(email string, excludeID uint) (bool, error) {
	f.emailCheckCount++
	f.lastEmailExcludeID = excludeID
	if f.emailExistsErr != nil {
		return false, f.emailExistsErr
	}
	if f.forceEmailConflict {
		return true, nil
	}
	if f.conflictAfterFirstCheck && f.emailCheckCount > 1 {
		return true, nil
	}
	for id, user := range f.users {
		if id != excludeID && user.Email != nil && *user.Email == email {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeAdminCRUDStore) GetByID(id uint) (*model.AdminUser, error) {
	user, ok := f.users[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copied := *user
	return &copied, nil
}

func (f *fakeAdminCRUDStore) GetList(_ int, _ int, _ string, status *int8) ([]*model.AdminUser, int64, error) {
	if status != nil {
		copied := *status
		f.lastListStatus = &copied
	}
	users := make([]*model.AdminUser, 0, len(f.users))
	for _, user := range f.users {
		copied := *user
		users = append(users, &copied)
	}
	return users, int64(len(users)), nil
}

func (f *fakeAdminCRUDStore) Update(user *model.AdminUser) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	copied := *user
	f.users[user.ID] = &copied
	return nil
}

func (f *fakeAdminCRUDStore) UpdatePassword(id uint, hashedPassword string) error {
	f.lastPasswordUserID = id
	f.lastHashedPassword = hashedPassword
	return nil
}

func (f *fakeAdminCRUDStore) UsernameExists(username string) bool {
	for _, user := range f.users {
		if user.Username == username {
			return true
		}
	}
	return false
}

func TestAdminUserServiceCreateNormalizesEmailAndStatus(t *testing.T) {
	tests := []struct {
		name       string
		status     *int8
		wantStatus int8
	}{
		{name: "missing status defaults enabled", wantStatus: 1},
		{name: "explicit zero stays disabled", status: adminStatusPointer(0), wantStatus: 0},
		{name: "legacy two becomes disabled", status: adminStatusPointer(2), wantStatus: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeAdminCRUDStore()
			service := newAdminUserService(store)
			response, err := service.CreateUser(&dto.CreateAdminUserRequest{
				Username: "new-admin",
				Password: "secret12",
				RealName: "New Admin",
				Email:    "  Admin@Example.COM  ",
				Status:   tt.status,
			})
			if err != nil {
				t.Fatalf("CreateUser() error = %v", err)
			}
			if response.Email == nil || *response.Email != "admin@example.com" {
				t.Fatalf("response email = %v, want normalized email", response.Email)
			}
			if response.AuthSource != adminAuthSourceLocal || response.OIDCBound {
				t.Fatalf("response identity = (%q, %v), want local/unbound", response.AuthSource, response.OIDCBound)
			}
			if response.Status != tt.wantStatus {
				t.Fatalf("response status = %d, want %d", response.Status, tt.wantStatus)
			}
			persisted := store.users[response.ID]
			if persisted.Email == nil || *persisted.Email != "admin@example.com" {
				t.Fatalf("persisted email = %v", persisted.Email)
			}
			if persisted.AuthSource != adminAuthSourceLocal || persisted.Status != tt.wantStatus {
				t.Fatalf("persisted identity/status = (%q, %d)", persisted.AuthSource, persisted.Status)
			}
		})
	}
}

func TestAdminUserServiceCreateValidatesEmailAndConflicts(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		configure func(*fakeAdminCRUDStore)
		wantErr   error
	}{
		{name: "empty", email: "   ", wantErr: domain.ErrInvalidAdminEmail},
		{name: "invalid", email: "not-an-email", wantErr: domain.ErrInvalidAdminEmail},
		{
			name:  "preflight conflict",
			email: "used@example.com",
			configure: func(store *fakeAdminCRUDStore) {
				store.forceEmailConflict = true
			},
			wantErr: domain.ErrAdminEmailConflict,
		},
		{
			name:  "database race conflict",
			email: "race@example.com",
			configure: func(store *fakeAdminCRUDStore) {
				store.createErr = errors.New("UNIQUE constraint failed: admin_users.email")
			},
			wantErr: domain.ErrAdminEmailConflict,
		},
		{
			name:  "translated database race conflict",
			email: "translated@example.com",
			configure: func(store *fakeAdminCRUDStore) {
				store.createErr = gorm.ErrDuplicatedKey
				store.conflictAfterFirstCheck = true
			},
			wantErr: domain.ErrAdminEmailConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeAdminCRUDStore()
			if tt.configure != nil {
				tt.configure(store)
			}
			service := newAdminUserService(store)
			_, err := service.CreateUser(&dto.CreateAdminUserRequest{
				Username: "new-admin",
				Password: "secret12",
				RealName: "New Admin",
				Email:    tt.email,
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CreateUser() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdminUserServiceUpdateUsernameEmailAndStatus(t *testing.T) {
	oldEmail := "old@example.com"
	store := newFakeAdminCRUDStore(&model.AdminUser{
		ID: 1, Username: "local", RealName: "Local", Email: &oldEmail, AuthSource: adminAuthSourceLocal, Status: 1,
	})
	service := newAdminUserService(store)

	newEmail := "  NEW@Example.COM "
	if err := service.UpdateUser(1, &dto.UpdateAdminUserRequest{
		Username: "renamed",
		RealName: "Renamed Admin",
		Email:    &newEmail,
	}); err != nil {
		t.Fatalf("UpdateUser(profile) error = %v", err)
	}
	if store.lastEmailExcludeID != 1 {
		t.Fatalf("email conflict excluded id = %d, want 1", store.lastEmailExcludeID)
	}
	if store.users[1].Username != "renamed" || store.users[1].RealName != "Renamed Admin" {
		t.Fatalf("updated username/real name = (%q, %q)", store.users[1].Username, store.users[1].RealName)
	}
	if store.users[1].Email == nil || *store.users[1].Email != "new@example.com" {
		t.Fatalf("updated email = %v", store.users[1].Email)
	}
	if store.users[1].Status != 1 {
		t.Fatalf("missing status changed value to %d", store.users[1].Status)
	}

	if err := service.UpdateUser(1, &dto.UpdateAdminUserRequest{Status: adminStatusPointer(0)}); err != nil {
		t.Fatalf("UpdateUser(status=0) error = %v", err)
	}
	if store.users[1].Status != 0 {
		t.Fatalf("explicit status zero persisted as %d", store.users[1].Status)
	}

	store.users[1].Status = 1
	if err := service.UpdateUser(1, &dto.UpdateAdminUserRequest{Status: adminStatusPointer(2)}); err != nil {
		t.Fatalf("UpdateUser(status=2) error = %v", err)
	}
	if store.users[1].Status != 0 {
		t.Fatalf("legacy status two persisted as %d", store.users[1].Status)
	}
}

func TestAdminUserServiceUpdateUsernameConflicts(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*fakeAdminCRUDStore)
	}{
		{
			name: "preflight conflict",
			configure: func(store *fakeAdminCRUDStore) {
				store.users[2] = &model.AdminUser{ID: 2, Username: "occupied"}
			},
		},
		{
			name: "database unique race",
			configure: func(store *fakeAdminCRUDStore) {
				store.updateErr = errors.New("UNIQUE constraint failed: admin_users.username")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeAdminCRUDStore(&model.AdminUser{
				ID: 1, Username: "original", RealName: "Original", AuthSource: adminAuthSourceLocal, Status: 1,
			})
			tt.configure(store)
			service := newAdminUserService(store)

			err := service.UpdateUser(1, &dto.UpdateAdminUserRequest{Username: "occupied"})
			if !errors.Is(err, domain.ErrAdminUsernameConflict) {
				t.Fatalf("UpdateUser() error = %v, want ErrAdminUsernameConflict", err)
			}
			if store.users[1].Username != "original" {
				t.Fatalf("failed update changed username to %q", store.users[1].Username)
			}
		})
	}
}

func TestAdminUserServiceUpdateEmailConflicts(t *testing.T) {
	oldEmail := "old@example.com"
	tests := []struct {
		name      string
		configure func(*fakeAdminCRUDStore)
	}{
		{
			name: "preflight conflict",
			configure: func(store *fakeAdminCRUDStore) {
				store.forceEmailConflict = true
			},
		},
		{
			name: "database unique race",
			configure: func(store *fakeAdminCRUDStore) {
				store.updateErr = errors.New("UNIQUE constraint failed: admin_users.email")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeAdminCRUDStore(&model.AdminUser{
				ID: 1, Username: "local", RealName: "Local", Email: &oldEmail, AuthSource: adminAuthSourceLocal, Status: 1,
			})
			tt.configure(store)
			service := newAdminUserService(store)
			newEmail := "new@example.com"
			err := service.UpdateUser(1, &dto.UpdateAdminUserRequest{Email: &newEmail})
			if !errors.Is(err, domain.ErrAdminEmailConflict) {
				t.Fatalf("UpdateUser() error = %v, want ErrAdminEmailConflict", err)
			}
			if store.users[1].Email == nil || *store.users[1].Email != oldEmail {
				t.Fatalf("failed update changed email to %v", store.users[1].Email)
			}
		})
	}
}

func TestAdminUserServiceUpdateHistoricalEmptyEmail(t *testing.T) {
	store := newFakeAdminCRUDStore(&model.AdminUser{
		ID: 1, Username: "legacy", RealName: "Legacy", AuthSource: adminAuthSourceLocal, Status: 1,
	})
	service := newAdminUserService(store)
	empty := "   "
	if err := service.UpdateUser(1, &dto.UpdateAdminUserRequest{RealName: "Legacy User", Email: &empty}); err != nil {
		t.Fatalf("historical empty email update error = %v", err)
	}
	if store.users[1].Email != nil {
		t.Fatalf("historical empty email = %v, want nil", store.users[1].Email)
	}

	existing := "set@example.com"
	store.users[1].Email = &existing
	if err := service.UpdateUser(1, &dto.UpdateAdminUserRequest{Email: &empty}); !errors.Is(err, domain.ErrInvalidAdminEmail) {
		t.Fatalf("clearing populated email error = %v, want ErrInvalidAdminEmail", err)
	}
}

func TestAdminUserServiceOIDCProfileUpdateAndPasswordRestriction(t *testing.T) {
	oidcEmail := "oidc@example.com"
	oidcSub := "subject"
	store := newFakeAdminCRUDStore(
		&model.AdminUser{
			ID: 1, Username: "oidc", RealName: "OIDC", Email: &oidcEmail, OidcSub: &oidcSub,
			AuthSource: adminAuthSourceOIDC, Status: 1,
		},
		&model.AdminUser{
			ID: 2, Username: "local-bound", RealName: "Local", OidcSub: &oidcSub,
			AuthSource: adminAuthSourceLocal, Status: 1,
		},
	)
	service := newAdminUserService(store)

	replacement := "replacement@example.com"
	if err := service.UpdateUser(1, &dto.UpdateAdminUserRequest{
		Username: "renamed-oidc",
		RealName: "Renamed OIDC",
		Email:    &replacement,
	}); err != nil {
		t.Fatalf("OIDC profile update error = %v", err)
	}
	if store.users[1].Username != "renamed-oidc" ||
		store.users[1].RealName != "Renamed OIDC" ||
		store.users[1].Email == nil ||
		*store.users[1].Email != replacement {
		t.Fatalf("OIDC profile was not updated: %+v", store.users[1])
	}
	if _, err := service.ResetPassword(1, &dto.ResetPasswordRequest{Password: "newSecret"}); !errors.Is(err, domain.ErrAdminPasswordResetForbidden) {
		t.Fatalf("OIDC reset error = %v, want forbidden", err)
	}

	response, err := service.ResetPassword(2, &dto.ResetPasswordRequest{Password: "newSecret"})
	if err != nil {
		t.Fatalf("local bound reset error = %v", err)
	}
	if response.Password != "newSecret" || store.lastPasswordUserID != 2 {
		t.Fatalf("local bound reset response/id = (%q, %d)", response.Password, store.lastPasswordUserID)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(store.lastHashedPassword), []byte("newSecret")); err != nil {
		t.Fatalf("stored password is not expected bcrypt hash: %v", err)
	}
}

func TestAdminUserServiceUnknownAuthSourceFailsClosed(t *testing.T) {
	unknownEmail := "unknown@example.com"
	store := newFakeAdminCRUDStore(
		&model.AdminUser{
			ID: 1, Username: "unknown", RealName: "Unknown", Email: &unknownEmail,
			AuthSource: "unexpected-provider", Status: 1,
		},
		&model.AdminUser{
			ID: 2, Username: "legacy", RealName: "Legacy", AuthSource: "", Status: 1,
		},
	)
	service := newAdminUserService(store)

	replacement := "replacement@example.com"
	if err := service.UpdateUser(1, &dto.UpdateAdminUserRequest{Email: &replacement}); err != nil {
		t.Fatalf("unknown auth source profile update error = %v", err)
	}
	if _, err := service.ResetPassword(1, &dto.ResetPasswordRequest{Password: "newSecret"}); !errors.Is(err, domain.ErrAdminPasswordResetForbidden) {
		t.Fatalf("unknown auth source reset error = %v, want forbidden", err)
	}
	if store.lastPasswordUserID != 0 {
		t.Fatalf("unknown auth source updated password for user %d", store.lastPasswordUserID)
	}
	response, err := service.GetUserByID(1)
	if err != nil {
		t.Fatalf("unknown auth source response error = %v", err)
	}
	if response.AuthSource != adminAuthSourceOIDC {
		t.Fatalf("unknown auth source response = %q, want fail-closed oidc", response.AuthSource)
	}

	if _, err := service.ResetPassword(2, &dto.ResetPasswordRequest{Password: "newSecret"}); err != nil {
		t.Fatalf("legacy empty auth source reset error = %v", err)
	}
	if store.lastPasswordUserID != 2 {
		t.Fatalf("legacy empty auth source password user = %d, want 2", store.lastPasswordUserID)
	}
}

func TestAdminUserServiceResponseIdentityAndLegacyStatus(t *testing.T) {
	email := "admin@example.com"
	sub := "subject"
	store := newFakeAdminCRUDStore(
		&model.AdminUser{
			ID: 1, Username: "admin", RealName: "Admin", Email: &email, OidcSub: &sub, AuthSource: "", Status: 2,
		},
		&model.AdminUser{
			ID: 2, Username: "legacy", RealName: "Legacy", AuthSource: adminAuthSourceLocal, Status: 1,
		},
	)
	service := newAdminUserService(store)
	response, err := service.GetUserByID(1)
	if err != nil {
		t.Fatalf("GetUserByID() error = %v", err)
	}
	if response.AuthSource != adminAuthSourceLocal || !response.OIDCBound {
		t.Fatalf("identity response = (%q, %v)", response.AuthSource, response.OIDCBound)
	}
	if response.Status != 0 {
		t.Fatalf("legacy stored status response = %d, want 0", response.Status)
	}
	if response.Email == nil || *response.Email != email {
		t.Fatalf("email response = %v", response.Email)
	}
	legacyResponse, err := service.GetUserByID(2)
	if err != nil {
		t.Fatalf("GetUserByID(legacy) error = %v", err)
	}
	if legacyResponse.Email != nil {
		t.Fatalf("legacy email response = %v, want nil/null", legacyResponse.Email)
	}
}

func TestAdminUserServiceListNormalizesLegacyStatusFilter(t *testing.T) {
	store := newFakeAdminCRUDStore()
	service := newAdminUserService(store)
	if _, err := service.GetUserList(&dto.AdminUserListRequest{Status: adminStatusPointer(2)}); err != nil {
		t.Fatalf("GetUserList(status=2) error = %v", err)
	}
	if store.lastListStatus == nil || *store.lastListStatus != 0 {
		t.Fatalf("list status = %v, want 0", store.lastListStatus)
	}
}

func TestDuplicateKeyDetectionAcrossDialects(t *testing.T) {
	messages := []string{
		"Error 1062 (23000): Duplicate entry for key idx_admin_users_email",
		"ERROR: duplicate key value violates unique constraint (SQLSTATE 23505)",
		"UNIQUE constraint failed: admin_users.email",
	}
	for _, message := range messages {
		t.Run(strings.Split(message, " ")[0], func(t *testing.T) {
			if !isDuplicateKeyError(errors.New(message)) {
				t.Fatalf("isDuplicateKeyError(%q) = false", message)
			}
		})
	}
}

func adminStatusPointer(value int8) *int8 {
	return &value
}
