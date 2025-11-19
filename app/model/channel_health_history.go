package model

import (
	"time"
)

// ChannelHealthHistory 通道健康历史记录
type ChannelHealthHistory struct {
	ID                uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ProviderChannelID uint      `gorm:"type:bigint unsigned;not null;index:idx_channel_time;comment:服务商通道ID" json:"provider_channel_id"`
	CheckTime         time.Time `gorm:"type:timestamp;not null;index:idx_channel_time,idx_check_time;comment:检查时间" json:"check_time"`
	Status            string    `gorm:"type:varchar(20);not null;comment:状态：healthy, unhealthy" json:"status"`
	ResponseTime      int       `gorm:"type:int;comment:响应时间（毫秒）" json:"response_time"`
	ErrorCount        int       `gorm:"type:int;default:0;comment:滑窗错误数" json:"error_count"`
	SuccessRate       float64   `gorm:"type:decimal(5,2);comment:成功率（%）" json:"success_rate"`
	IsAvailable       int8      `gorm:"type:tinyint;default:1;comment:是否可用：1=是 0=否" json:"is_available"`
	CreatedAt         time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
}

// TableName 指定表名
func (ChannelHealthHistory) TableName() string {
	return "channel_health_history"
}
