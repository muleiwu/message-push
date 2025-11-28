package model

import (
	"time"
)

// AppQuotaStat 应用配额统计表
type AppQuotaStat struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	AppID        string    `gorm:"type:varchar(32);not null;uniqueIndex:uk_app_date;comment:应用ID" json:"app_id"`
	StatDate     time.Time `gorm:"type:date;not null;uniqueIndex:uk_app_date;index:idx_stat_date;comment:统计日期" json:"stat_date"`
	TotalCount   int       `gorm:"type:int;default:0;comment:总发送数" json:"total_count"`
	SuccessCount int       `gorm:"type:int;default:0;comment:成功数" json:"success_count"`
	FailedCount  int       `gorm:"type:int;default:0;comment:失败数" json:"failed_count"`
	CreatedAt    time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
}

// TableName 指定表名
func (AppQuotaStat) TableName() string {
	return "app_quota_stats"
}
