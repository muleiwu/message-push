package dto

import (
	"encoding/json"
	"testing"
)

func TestAdminUserStatusRequestsPreserveOmittedAndExplicitDisabled(t *testing.T) {
	t.Run("create omitted", func(t *testing.T) {
		var req CreateAdminUserRequest
		if err := json.Unmarshal([]byte(`{"username":"admin","password":"secret12","real_name":"Admin","email":"admin@example.com"}`), &req); err != nil {
			t.Fatalf("unmarshal create request: %v", err)
		}
		if req.Status != nil {
			t.Fatalf("omitted create status = %v, want nil", req.Status)
		}
	})

	t.Run("create explicit zero", func(t *testing.T) {
		var req CreateAdminUserRequest
		if err := json.Unmarshal([]byte(`{"status":0}`), &req); err != nil {
			t.Fatalf("unmarshal create request: %v", err)
		}
		if req.Status == nil || *req.Status != 0 {
			t.Fatalf("explicit create status = %v, want 0", req.Status)
		}
	})

	t.Run("update legacy two", func(t *testing.T) {
		var req UpdateAdminUserRequest
		if err := json.Unmarshal([]byte(`{"username":"renamed_admin","status":2}`), &req); err != nil {
			t.Fatalf("unmarshal update request: %v", err)
		}
		if req.Username != "renamed_admin" {
			t.Fatalf("update username = %q, want renamed_admin", req.Username)
		}
		if req.Status == nil || *req.Status != 2 {
			t.Fatalf("legacy update status = %v, want 2 for service normalization", req.Status)
		}
	})
}
