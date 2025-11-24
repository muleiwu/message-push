package model

import (
	"time"

	"gorm.io/gorm"
)

// ProviderSignature 供应商签名表
type ProviderSignature struct {
	ID                uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	ProviderAccountID uint             `gorm:"type:bigint unsigned;not null;index:idx_provider_account;comment:供应商账号ID（关联provider_accounts表）" json:"provider_account_id"`
	SignatureCode     string           `gorm:"type:varchar(100);not null;comment:签名代码（用于API调用，如阿里云/腾讯云的签名标识）" json:"signature_code"`
	SignatureName     string           `gorm:"type:varchar(100);not null;comment:签名名称（显示用）" json:"signature_name"`
	Status            int8             `gorm:"type:tinyint;default:1;index:idx_status;comment:状态：1=启用 0=禁用" json:"status"`
	IsDefault         int8             `gorm:"type:tinyint;default:0;index:idx_provider_default;comment:是否默认签名：1=是 0=否" json:"is_default"`
	Remark            string           `gorm:"type:text;comment:备注说明" json:"remark"`
	CreatedAt         time.Time        `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt         time.Time        `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt         gorm.DeletedAt   `gorm:"index" json:"deleted_at"`
	ProviderAccount   *ProviderAccount `gorm:"foreignKey:ProviderAccountID;references:ID" json:"provider_account,omitempty"`
}

// TableName 指定表名
func (ProviderSignature) TableName() string {
	return "provider_signatures"
}
