package timeutil

import (
	"testing"
	"time"
)

func TestNormalizeAndFormatUTC(t *testing.T) {
	shanghai := time.FixedZone("UTC+8", 8*60*60)
	input := time.Date(2026, 8, 4, 18, 0, 0, 0, shanghai)

	got := Normalize(input)
	want := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("Normalize(%v) = %v (%v), want %v UTC", input, got, got.Location(), want)
	}
	if formatted := FormatRFC3339(input); formatted != "2026-08-04T10:00:00Z" {
		t.Fatalf("FormatRFC3339(%v) = %q", input, formatted)
	}
}

func TestBusinessDayRangeUsesAsiaShanghai(t *testing.T) {
	start, end, err := BusinessDateRangeUTC("2026-08-04", "2026-08-04")
	if err != nil {
		t.Fatalf("BusinessDateRangeUTC() error = %v", err)
	}
	wantStart := time.Date(2026, 8, 3, 16, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 8, 4, 16, 0, 0, 0, time.UTC)
	if !start.Equal(wantStart) || !end.Equal(wantEnd) {
		t.Fatalf("range = [%v, %v), want [%v, %v)", start, end, wantStart, wantEnd)
	}
}

func TestBusinessDateChangesAtShanghaiMidnight(t *testing.T) {
	before := time.Date(2026, 8, 4, 15, 59, 59, 0, time.UTC)
	after := time.Date(2026, 8, 4, 16, 0, 0, 0, time.UTC)
	if got := BusinessDate(before); got != "2026-08-04" {
		t.Fatalf("business date before midnight = %q", got)
	}
	if got := BusinessDate(after); got != "2026-08-05" {
		t.Fatalf("business date at midnight = %q", got)
	}
}
