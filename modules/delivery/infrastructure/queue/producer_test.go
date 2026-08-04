package queue

import (
	"testing"
	"time"
)

func TestScheduledInstantComparisonIgnoresInputOffset(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	shanghai := time.FixedZone("UTC+8", 8*60*60)

	if isFutureInstant(time.Date(2026, 8, 4, 18, 0, 0, 0, shanghai), now) {
		t.Fatal("same absolute instant must not be delayed")
	}
	if !isFutureInstant(time.Date(2026, 8, 4, 18, 0, 1, 0, shanghai), now) {
		t.Fatal("one second after the absolute instant must be delayed")
	}
}
