package controller

import (
	"strings"
	"testing"

	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/dto"
)

type invalidInstallContext struct {
	httpInterfaces.RouterContextInterface
	request    dto.InstallSubmitRequest
	statusCode int
	response   any
}

func (c *invalidInstallContext) ShouldBindJSON(target any) error {
	request := target.(*dto.InstallSubmitRequest)
	*request = c.request
	return nil
}

func (c *invalidInstallContext) JSON(statusCode int, response any) {
	c.statusCode = statusCode
	c.response = response
}

func TestNormalizeInstallAdmin(t *testing.T) {
	admin := dto.AdminAccountInfo{
		Username: "installer",
		Password: "install-password",
		RealName: "Initial Admin",
		Email:    "  Installer@Example.COM ",
	}
	normalized, err := normalizeInstallAdmin(admin)
	if err != nil {
		t.Fatalf("normalizeInstallAdmin() error = %v", err)
	}
	if normalized.Email != "installer@example.com" {
		t.Fatalf("normalized email = %q, want installer@example.com", normalized.Email)
	}

	for _, invalid := range []dto.AdminAccountInfo{
		{Password: "password", RealName: "Admin", Email: "admin@example.com"},
		{Username: "admin", Password: "password", RealName: "Admin", Email: "   "},
		{Username: "admin", Password: "password", RealName: "Admin", Email: "用户@example.com"},
	} {
		if _, err := normalizeInstallAdmin(invalid); err == nil {
			t.Fatalf("normalizeInstallAdmin(%+v) error = nil", invalid)
		}
	}
}

func TestSubmitInstallRejectsInvalidAdminBeforeSideEffects(t *testing.T) {
	ctx := &invalidInstallContext{request: dto.InstallSubmitRequest{
		Admin: dto.AdminAccountInfo{
			Username: "installer",
			Password: "install-password",
			RealName: "Initial Admin",
			Email:    "not-an-email",
		},
	}}

	InstallController{}.SubmitInstall(ctx)

	if ctx.statusCode != 400 {
		t.Fatalf("HTTP status = %d, want 400", ctx.statusCode)
	}
	response, ok := ctx.response.(dto.Response)
	if !ok {
		t.Fatalf("response type = %T, want dto.Response", ctx.response)
	}
	if response.Code != 400 || !strings.Contains(response.Message, "管理员邮箱无效") {
		t.Fatalf("response = %+v, want validation error", response)
	}
}
