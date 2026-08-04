package dto

import (
	"encoding/json"
	"testing"
	"time"
)

func TestScheduledAtAcceptsEquivalentRFC3339Offsets(t *testing.T) {
	decode := func(value string) time.Time {
		t.Helper()
		var request SendRequest
		if err := json.Unmarshal([]byte(`{"scheduled_at":"`+value+`"}`), &request); err != nil {
			t.Fatalf("decode scheduled_at %q: %v", value, err)
		}
		if request.ScheduledAt == nil {
			t.Fatalf("scheduled_at %q decoded as nil", value)
		}
		return *request.ScheduledAt
	}

	utc := decode("2026-08-04T10:00:00Z")
	shanghai := decode("2026-08-04T18:00:00+08:00")
	if !utc.Equal(shanghai) {
		t.Fatalf("equivalent scheduled_at values differ: %s vs %s", utc, shanghai)
	}
}
