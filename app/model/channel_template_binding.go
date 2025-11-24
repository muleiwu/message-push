package model

import (
	"time"
)

// ChannelTemplateBinding 通道模板绑定配置表
type ChannelTemplateBinding struct {
	ID                   uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	ChannelID            uint             `gorm:"type:bigint unsigned;not null;index:idx_channel;comment:通道ID（关联channels表）" json:"channel_id"`
	TemplateBindingID    uint             `gorm:"type:bigint unsigned;not null;index:idx_template_binding;comment:模板绑定ID（关联template_bindings表）" json:"template_binding_id"`
	Weight               int              `gorm:"type:int;default:10;comment:权重（同优先级下按权重分配流量）" json:"weight"`
	Priority             int              `gorm:"type:int;default:100;comment:优先级（数字越小越优先）" json:"priority"`
	IsActive             int8             `gorm:"type:tinyint;default:1;comment:是否激活：1=是 0=否" json:"is_active"`
	AutoDisableOnFail    bool             `gorm:"type:tinyint;default:0;comment:失败时自动禁用" json:"auto_disable_on_fail"`
	AutoDisableThreshold int              `gorm:"type:int;default:5;comment:自动禁用阈值（连续失败次数）" json:"auto_disable_threshold"`
	CreatedAt            time.Time        `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt            time.Time        `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	Channel              *Channel         `gorm:"foreignKey:ChannelID;references:ID" json:"channel,omitempty"`
	TemplateBinding      *TemplateBinding `gorm:"foreignKey:TemplateBindingID;references:ID" json:"template_binding,omitempty"`
}

// TableName 指定表名
func (ChannelTemplateBinding) TableName() string {
	return "channel_template_bindings"
}
