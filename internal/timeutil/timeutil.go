// Package timeutil centralizes the service's time policy: instants use UTC,
// while business calendar days use Asia/Shanghai.
package timeutil

import (
	"fmt"
	"time"
)

const (
	BusinessTimeZone = "Asia/Shanghai"
	DateLayout       = "2006-01-02"
)

var businessLocation = mustLoadBusinessLocation()

func mustLoadBusinessLocation() *time.Location {
	location, err := time.LoadLocation(BusinessTimeZone)
	if err != nil {
		panic(fmt.Sprintf("load business timezone %s: %v", BusinessTimeZone, err))
	}
	return location
}

func Now() time.Time { return time.Now().UTC() }

func Normalize(value time.Time) time.Time { return value.UTC() }

func NormalizePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := Normalize(*value)
	return &normalized
}

func FormatRFC3339(value time.Time) string {
	return Normalize(value).Format(time.RFC3339)
}

func BusinessLocation() *time.Location { return businessLocation }

func ParseBusinessTime(layout, value string) (time.Time, error) {
	parsed, err := time.ParseInLocation(layout, value, businessLocation)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func BusinessDate(value time.Time) string {
	return value.In(businessLocation).Format(DateLayout)
}

func BusinessDayStart(value time.Time) time.Time {
	local := value.In(businessLocation)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, businessLocation)
}

// BusinessDateRangeUTC converts inclusive Asia/Shanghai calendar dates to a
// half-open UTC instant range [start, end).
func BusinessDateRangeUTC(startDate, endDate string) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation(DateLayout, startDate, businessLocation)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start date %q: %w", startDate, err)
	}
	end, err := time.ParseInLocation(DateLayout, endDate, businessLocation)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end date %q: %w", endDate, err)
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end date %q is before start date %q", endDate, startDate)
	}
	return start.UTC(), end.AddDate(0, 0, 1).UTC(), nil
}
