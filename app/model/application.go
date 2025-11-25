package model

import (
	"time"

	"gorm.io/gorm"
)

// Application 应用管理表
type Application struct {
	ID          uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	AppID       string         `gorm:"type:varchar(32);uniqueIndex:uk_app_id;not null" json:"app_id"`
	AppSecret   string         `gorm:"type:varchar(128);not null;comment:应用密钥（加密存储）" json:"app_secret"`
	AppName     string         `gorm:"type:varchar(100);not null" json:"app_name"`
	Status      int8           `gorm:"type:tinyint;default:1;index:idx_status;comment:状态：1=启用 0=禁用" json:"status"`
	IPWhitelist string         `gorm:"type:text;comment:IP白名单，换行分隔，支持IP和CIDR子网格式，空表示不限制" json:"ip_whitelist"`
	WebhookURL  string         `gorm:"type:varchar(255);comment:异步回调通知地址" json:"webhook_url"`
	DailyQuota  int            `gorm:"type:int;default:10000;comment:每日发送配额" json:"daily_quota"`
	RateLimit   int            `gorm:"type:int;default:100;comment:每秒速率限制（QPS）" json:"rate_limit"`
	CreatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// TableName 指定表名
func (Application) TableName() string {
	return "applications"
}
