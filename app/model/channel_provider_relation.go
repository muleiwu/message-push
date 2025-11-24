package model

import (
	"time"
)

// ChannelProviderRelation 推送通道关联表（含签名/模板配置）
type ChannelProviderRelation struct {
	ID                uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	ChannelID         uint             `gorm:"type:bigint unsigned;not null;index:idx_channel;comment:通道ID" json:"channel_id"`
	ProviderChannelID uint             `gorm:"type:bigint unsigned;not null;index:idx_provider_channel;comment:服务商通道ID" json:"provider_channel_id"`
	Priority          int              `gorm:"type:int;default:10;index:idx_priority_weight;comment:优先级（数字越小越优先）" json:"priority"`
	Weight            int              `gorm:"type:int;default:1;index:idx_priority_weight;comment:权重（同优先级下按权重分配流量）" json:"weight"`
	IsActive          int8             `gorm:"type:tinyint;default:1;index:idx_push_channel;comment:是否激活：1=是 0=否" json:"is_active"`
	SignatureCode     string           `gorm:"type:varchar(50);comment:签名代码（短信使用）" json:"signature_code"`
	TemplateCode      string           `gorm:"type:varchar(50);comment:模板代码（如有）" json:"template_code"`
	TemplateID        string           `gorm:"type:varchar(100);comment:服务商模板ID（如SMS_123456）" json:"template_id"`
	TemplateParams    string           `gorm:"type:json;comment:模板参数定义（参数名、类型、默认值）" json:"template_params"`
	CreatedAt         time.Time        `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt         time.Time        `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	Channel           *Channel         `gorm:"foreignKey:ChannelID;references:ID" json:"channel,omitempty"`
	ProviderChannel   *ProviderChannel `gorm:"foreignKey:ProviderChannelID;references:ID" json:"provider_channel,omitempty"`
}

// TableName 指定表名
func (ChannelProviderRelation) TableName() string {
	return "channel_provider_relations"
}
