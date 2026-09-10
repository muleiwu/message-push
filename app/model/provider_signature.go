package model

import (
	"time"

	"gorm.io/gorm"
)

// ProviderSignature 供应商映射资源表（短信签名或邮件标题）
type ProviderSignature struct {
	ProviderResourceState
	RemoteID          string           `gorm:"type:varchar(100);default:''" json:"remote_id"`
	RemoteDescription string           `gorm:"type:text" json:"remote_description"`
	ID                uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	ProviderAccountID uint             `gorm:"not null;index:idx_provider_account;comment:供应商账号ID（关联provider_accounts表）" json:"provider_account_id"`
	SignatureCode     string           `gorm:"type:varchar(200);not null;comment:映射值（实际发送用：短信签名或邮件标题；短信须与供应商平台报备审核通过的签名一致）" json:"signature_code"`
	SignatureName     string           `gorm:"type:varchar(100);not null;comment:签名名称（仅后台展示用：不参与实际发送，仅用于后台识别。通常与签名代码相同）" json:"signature_name"`
	Status            int8             `gorm:"default:1;index:idx_status;comment:状态：1=启用 0=禁用" json:"status"`
	Remark            string           `gorm:"type:text;comment:备注说明" json:"remark"`
	CreatedAt         time.Time        `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt         time.Time        `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt         gorm.DeletedAt   `gorm:"index" json:"deleted_at"`
	ProviderAccount   *ProviderAccount `gorm:"foreignKey:ProviderAccountID;references:ID" json:"provider_account,omitempty"`
}

// TableName 指定表名
func (ProviderSignature) TableName() string {
	return "provider_signatures"
}
