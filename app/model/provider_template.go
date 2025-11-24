package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ProviderTemplate 供应商模板表
type ProviderTemplate struct {
	ID              uint             `gorm:"primaryKey;autoIncrement" json:"id"`
	ProviderID      uint             `gorm:"type:bigint unsigned;not null;index:idx_provider_status;comment:供应商账号ID（关联provider_accounts表）" json:"provider_id"`
	TemplateCode    string           `gorm:"type:varchar(100);not null;comment:供应商模板代码（如阿里云SMS_123456789）" json:"template_code"`
	TemplateName    string           `gorm:"type:varchar(200);not null;comment:供应商模板名称" json:"template_name"`
	TemplateContent string           `gorm:"type:text;comment:供应商模板内容（如：验证码$${code}）" json:"template_content"`
	Variables       string           `gorm:"type:json;comment:供应商模板变量列表，JSON数组格式" json:"variables"`
	Status          int8             `gorm:"type:tinyint;default:1;index:idx_provider_status;comment:状态：1=启用 0=禁用" json:"status"`
	Remark          string           `gorm:"type:text;comment:备注说明" json:"remark"`
	CreatedAt       time.Time        `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt       time.Time        `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt       gorm.DeletedAt   `gorm:"index" json:"deleted_at"`
	ProviderAccount *ProviderAccount `gorm:"foreignKey:ProviderID;references:ID" json:"provider_account,omitempty"`
}

// TableName 指定表名
func (ProviderTemplate) TableName() string {
	return "provider_templates"
}

// GetVariables 获取变量列表（反序列化）
func (p *ProviderTemplate) GetVariables() ([]string, error) {
	var variables []string
	if p.Variables == "" {
		return variables, nil
	}
	err := json.Unmarshal([]byte(p.Variables), &variables)
	return variables, err
}

// SetVariables 设置变量列表（序列化）
func (p *ProviderTemplate) SetVariables(variables []string) error {
	data, err := json.Marshal(variables)
	if err != nil {
		return err
	}
	p.Variables = string(data)
	return nil
}
