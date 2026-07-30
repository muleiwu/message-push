package admin

import (
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
)

func TestBuildUserInfoResponse(t *testing.T) {
	localEmail := "local@example.com"
	oidcEmail := "sso@example.com"

	tests := []struct {
		name       string
		user       model.AdminUser
		wantEmail  *string
		wantSource string
	}{
		{
			name: "local account",
			user: model.AdminUser{
				ID:         1,
				Username:   "local-admin",
				RealName:   "本地管理员",
				Email:      &localEmail,
				AuthSource: "local",
			},
			wantEmail:  &localEmail,
			wantSource: "local",
		},
		{
			name: "oidc account",
			user: model.AdminUser{
				ID:         2,
				Username:   "sso-admin",
				RealName:   "SSO 管理员",
				Email:      &oidcEmail,
				AuthSource: "oidc",
			},
			wantEmail:  &oidcEmail,
			wantSource: "oidc",
		},
		{
			name: "legacy account without email",
			user: model.AdminUser{
				ID:         3,
				Username:   "legacy-admin",
				RealName:   "历史管理员",
				AuthSource: "local",
			},
			wantEmail:  nil,
			wantSource: "local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildUserInfoResponse(&tt.user)

			if got.UserID != tt.user.ID ||
				got.Username != tt.user.Username ||
				got.RealName != tt.user.RealName {
				t.Fatalf("identity fields = (%d, %q, %q), want (%d, %q, %q)",
					got.UserID, got.Username, got.RealName,
					tt.user.ID, tt.user.Username, tt.user.RealName,
				)
			}
			if got.Email != tt.wantEmail {
				t.Fatalf("email = %v, want %v", got.Email, tt.wantEmail)
			}
			if got.AuthSource != tt.wantSource {
				t.Fatalf("auth source = %q, want %q", got.AuthSource, tt.wantSource)
			}
			if len(got.Roles) != 1 || got.Roles[0] != "admin" {
				t.Fatalf("roles = %v, want [admin]", got.Roles)
			}
			if got.HomePath != "/dashboard" {
				t.Fatalf("home path = %q, want /dashboard", got.HomePath)
			}
		})
	}
}
