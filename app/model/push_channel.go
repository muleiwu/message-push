package model

import (
	"time"

	"gorm.io/gorm"
)

// PushChannel 推送通道表（业务层）
type PushChannel struct {
	ID                uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	ChannelCode       string           `gorm:"type:varchar(50);uniqueIndex:uk_channel_code;not null;comment:推送通道代码（业务使用）" json:"channel_code"`
	ChannelName       string           `gorm:"type:varchar(100);not null" json:"channel_name"`
	ChannelType       string           `gorm:"type:varchar(20);not null;index:idx_type_status;comment:类型：sms, email, wechat_work, dingtalk" json:"channel_type"`
	MessageTemplateID uint             `gorm:"type:bigint unsigned;index:idx_message_template;comment:绑定的系统模板ID" json:"message_template_id"`
	Status            int8             `gorm:"type:tinyint;default:1;index:idx_type_status;comment:状态：1=启用 0=禁用" json:"status"`
	Remark            string           `gorm:"type:text;comment:备注说明" json:"remark"`
	CreatedAt         time.Time        `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt         time.Time        `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt         gorm.DeletedAt   `gorm:"index" json:"deleted_at"`
	MessageTemplate   *MessageTemplate `gorm:"foreignKey:MessageTemplateID;references:ID" json:"message_template,omitempty"`
}

// TableName 指定表名
func (PushChannel) TableName() string {
	return "push_channels"
}
