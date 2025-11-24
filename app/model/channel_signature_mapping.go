package model

import (
	"time"

	"gorm.io/gorm"
)

// ChannelSignatureMapping 通道签名映射表
type ChannelSignatureMapping struct {
	ID                  uint               `gorm:"primaryKey;autoIncrement" json:"id"`
	ChannelID           uint               `gorm:"type:bigint unsigned;not null;index:idx_channel;comment:通道ID（关联channels表）" json:"channel_id"`
	SignatureName       string             `gorm:"type:varchar(100);not null;comment:用户自定义签名名称" json:"signature_name"`
	ProviderSignatureID uint               `gorm:"type:bigint unsigned;not null;comment:供应商签名ID（关联provider_signatures表）" json:"provider_signature_id"`
	ProviderID          uint               `gorm:"type:bigint unsigned;not null;index:idx_provider;comment:供应商账号ID（冗余字段，便于查询）" json:"provider_id"`
	Status              int8               `gorm:"type:tinyint;default:1;comment:状态：1=启用 0=禁用" json:"status"`
	CreatedAt           time.Time          `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt           time.Time          `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt           gorm.DeletedAt     `gorm:"index" json:"deleted_at"`
	Channel             *Channel           `gorm:"foreignKey:ChannelID;references:ID" json:"channel,omitempty"`
	ProviderSignature   *ProviderSignature `gorm:"foreignKey:ProviderSignatureID;references:ID" json:"provider_signature,omitempty"`
	ProviderAccount     *ProviderAccount   `gorm:"foreignKey:ProviderID;references:ID" json:"provider_account,omitempty"`
}

// TableName 指定表名
func (ChannelSignatureMapping) TableName() string {
	return "channel_signature_mappings"
}
