package infrastructure

import (
	"testing"
	"time"
)

func TestQuotaKeyUsesShanghaiBusinessDay(t *testing.T) {
	beforeBoundary := time.Date(2026, 8, 4, 15, 59, 59, 0, time.UTC)
	atBoundary := beforeBoundary.Add(time.Second)

	if got, want := quotaKeyAt(7, beforeBoundary), "quota:7:20260804"; got != want {
		t.Fatalf("quotaKeyAt(before boundary) = %q, want %q", got, want)
	}
	if got, want := quotaKeyAt(7, atBoundary), "quota:7:20260805"; got != want {
		t.Fatalf("quotaKeyAt(at boundary) = %q, want %q", got, want)
	}
}
