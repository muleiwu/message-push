package admin

import (
	"errors"
	"fmt"
	"testing"

	"cnb.cool/mliev/push/message-push/modules/identity"
)

func TestAdminUserServiceErrorStatus(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid email", err: identity.ErrInvalidAdminEmail, status: 400},
		{name: "not found", err: identity.ErrAdminUserNotFound, status: 404},
		{name: "username conflict", err: identity.ErrAdminUsernameConflict, status: 409},
		{name: "email conflict", err: fmt.Errorf("wrapped: %w", identity.ErrAdminEmailConflict), status: 409},
		{name: "legacy immutable oidc email", err: identity.ErrAdminEmailImmutable, status: 403},
		{name: "oidc password reset", err: identity.ErrAdminPasswordResetForbidden, status: 403},
		{name: "internal", err: errors.New("database unavailable"), status: 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := adminUserServiceErrorStatus(tt.err); got != tt.status {
				t.Fatalf("adminUserServiceErrorStatus() = %d, want %d", got, tt.status)
			}
		})
	}
}
