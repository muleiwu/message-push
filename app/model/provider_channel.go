package model

import (
	"time"

	"gorm.io/gorm"
)

// ProviderChannel 服务商通道表
type ProviderChannel struct {
	ID             uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	ProviderID     uint           `gorm:"type:bigint unsigned;not null;index:idx_provider_status;comment:服务商ID" json:"provider_id"`
	ChannelCode    string         `gorm:"type:varchar(50);uniqueIndex:uk_channel_code;not null" json:"channel_code"`
	ChannelName    string         `gorm:"type:varchar(100);not null" json:"channel_name"`
	Config         string         `gorm:"type:json;comment:通道级配置（可覆盖服务商配置）" json:"config"`
	HealthCheckURL string         `gorm:"type:varchar(255);comment:健康检查地址（可选）" json:"health_check_url"`
	MaxRetry       int            `gorm:"type:int;default:3;comment:最大重试次数" json:"max_retry"`
	RetryInterval  int            `gorm:"type:int;default:2;comment:重试间隔（秒）" json:"retry_interval"`
	Timeout        int            `gorm:"type:int;default:10;comment:超时时间（秒）" json:"timeout"`
	Status         int8           `gorm:"type:tinyint;default:1;index:idx_provider_status;comment:状态：1=启用 0=禁用" json:"status"`
	Remark         string         `gorm:"type:text;comment:备注说明" json:"remark"`
	CreatedAt      time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	Provider       *Provider      `gorm:"foreignKey:ProviderID;references:ID" json:"provider,omitempty"`
}

// TableName 指定表名
func (ProviderChannel) TableName() string {
	return "provider_channels"
}
