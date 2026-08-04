package dao

import (
	"fmt"

	"cnb.cool/mliev/push/message-push/internal/timeutil"
	"gorm.io/gorm"
)

// applyBusinessDateFilter converts inclusive Asia/Shanghai calendar dates to
// an index-friendly half-open UTC instant range.
func applyBusinessDateFilter(query *gorm.DB, column, startDate, endDate string) (*gorm.DB, error) {
	if startDate != "" {
		start, _, err := timeutil.BusinessDateRangeUTC(startDate, startDate)
		if err != nil {
			return nil, fmt.Errorf("invalid start_date: %w", err)
		}
		query = query.Where(column+" >= ?", start)
	}
	if endDate != "" {
		_, end, err := timeutil.BusinessDateRangeUTC(endDate, endDate)
		if err != nil {
			return nil, fmt.Errorf("invalid end_date: %w", err)
		}
		query = query.Where(column+" < ?", end)
	}
	return query, nil
}
