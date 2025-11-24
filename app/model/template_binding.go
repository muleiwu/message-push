package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// TemplateBinding 模板绑定表
type TemplateBinding struct {
	ID                 uint              `gorm:"primaryKey;autoIncrement" json:"id"`
	MessageTemplateID  uint              `gorm:"type:bigint unsigned;not null;index:idx_msg_template;comment:系统模板ID（关联message_templates表）" json:"message_template_id"`
	ProviderTemplateID uint              `gorm:"type:bigint unsigned;not null;comment:供应商模板ID（关联provider_templates表）" json:"provider_template_id"`
	ProviderID         uint              `gorm:"type:bigint unsigned;not null;index:idx_provider;comment:供应商账号ID（冗余字段，便于查询）" json:"provider_id"`
	ParamMapping       string            `gorm:"type:json;comment:参数映射，JSON对象格式 {\"系统变量\":\"供应商变量\"}" json:"param_mapping"`
	Status             int8              `gorm:"type:tinyint;default:1;index:idx_msg_template,idx_provider;comment:状态：1=启用 0=禁用" json:"status"`
	Priority           int               `gorm:"type:int;default:100;comment:优先级，数字越小优先级越高" json:"priority"`
	CreatedAt          time.Time         `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt          time.Time         `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt          gorm.DeletedAt    `gorm:"index" json:"deleted_at"`
	MessageTemplate    *MessageTemplate  `gorm:"foreignKey:MessageTemplateID;references:ID" json:"message_template,omitempty"`
	ProviderTemplate   *ProviderTemplate `gorm:"foreignKey:ProviderTemplateID;references:ID" json:"provider_template,omitempty"`
	ProviderAccount    *ProviderAccount  `gorm:"foreignKey:ProviderID;references:ID" json:"provider_account,omitempty"`
}

// TableName 指定表名
func (TemplateBinding) TableName() string {
	return "template_bindings"
}

// GetParamMapping 获取参数映射（反序列化）
func (t *TemplateBinding) GetParamMapping() (map[string]string, error) {
	var mapping map[string]string
	if t.ParamMapping == "" {
		return mapping, nil
	}
	err := json.Unmarshal([]byte(t.ParamMapping), &mapping)
	return mapping, err
}

// SetParamMapping 设置参数映射（序列化）
func (t *TemplateBinding) SetParamMapping(mapping map[string]string) error {
	data, err := json.Marshal(mapping)
	if err != nil {
		return err
	}
	t.ParamMapping = string(data)
	return nil
}
