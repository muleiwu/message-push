package admin

import (
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
)

func TestFailureRuleResponseUsesRFC3339UTC(t *testing.T) {
	shanghai := time.FixedZone("UTC+8", 8*60*60)
	rule := &model.FailureRule{
		CreatedAt: time.Date(2026, 8, 4, 18, 0, 0, 0, shanghai),
		UpdatedAt: time.Date(2026, 8, 4, 18, 5, 0, 0, shanghai),
	}

	response := toFailureRuleResponse(rule)
	if response.CreatedAt != "2026-08-04T10:00:00Z" || response.UpdatedAt != "2026-08-04T10:05:00Z" {
		t.Fatalf("failure rule times = %q/%q", response.CreatedAt, response.UpdatedAt)
	}
}
