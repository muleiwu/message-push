package infrastructure

import "testing"

func TestNormalizeEmailSubject(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "default empty", input: "", want: "通知"},
		{name: "default whitespace", input: "  \t ", want: "通知"},
		{name: "trim normal", input: "  订单通知  ", want: "订单通知"},
		{name: "reject CRLF injection", input: "正常主题\r\nBcc: attacker@example.com", wantErr: true},
		{name: "reject bare carriage return", input: "主题\rBcc: attacker@example.com", wantErr: true},
		{name: "reject bare newline", input: "主题\nBcc: attacker@example.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeEmailSubject(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeEmailSubject(%q) = %q, want error", tt.input, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("normalizeEmailSubject(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
			}
		})
	}
}
