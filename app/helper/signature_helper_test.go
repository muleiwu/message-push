package helper

import "testing"

func TestSignatureHelperMatchesManualExample(t *testing.T) {
	body := map[string]interface{}{
		"channel_id":     1,
		"receiver":       "+8613800000001",
		"signature_name": "木雷演示",
		"template_params": map[string]interface{}{
			"code":    "123456",
			"minutes": "5",
		},
	}
	got := NewSignatureHelper().GenerateSignature(
		"demo-shop-secret-2026",
		"POST",
		"/api/v1/messages",
		body,
		1784746800,
		"demo-nonce-001",
	)
	const want = "24904ae3656aaaf1e8e1f4df7d294f9dd2968c8957da6502f3a9b994a494ab64"
	if got != want {
		t.Fatalf("manual signature = %s, want %s", got, want)
	}
}
