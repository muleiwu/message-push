package helper

import (
	"strings"
	"testing"
)

func TestNormalizeAdminEmail(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "normalizes case and whitespace", input: "  Admin.User+Ops@Example.COM  ", want: "admin.user+ops@example.com"},
		{name: "allows empty for legacy accounts", input: " \t ", want: ""},
		{name: "rejects display name", input: "Admin <admin@example.com>", wantErr: true},
		{name: "rejects missing domain suffix", input: "admin@example", wantErr: true},
		{name: "rejects malformed address", input: "admin@@example.com", wantErr: true},
		{name: "rejects unicode local part", input: "用户@example.com", wantErr: true},
		{name: "rejects unicode domain", input: "admin@例子.com", wantErr: true},
		{name: "rejects over 255 characters", input: strings.Repeat("a", 244) + "@example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeAdminEmail(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeAdminEmail(%q) error = nil, want error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeAdminEmail(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeAdminEmail(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
