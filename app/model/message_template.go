package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// MessageTemplate 系统模板表
type MessageTemplate struct {
	ID           uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateName string         `gorm:"type:varchar(200);not null;comment:模板名称" json:"template_name"`
	MessageType  string         `gorm:"type:varchar(20);not null;index:idx_type_status;comment:消息类型：sms, email, wechat_work, dingtalk, webhook, push" json:"message_type"`
	Content      string         `gorm:"type:text;not null;comment:模板内容，使用{variable}占位符" json:"content"`
	Variables    string         `gorm:"type:json;comment:模板变量列表，JSON数组格式" json:"variables"`
	Description  string         `gorm:"type:text;comment:模板描述" json:"description"`
	Status       int8           `gorm:"type:tinyint;default:1;index:idx_type_status;comment:状态：1=启用 0=禁用" json:"status"`
	CreatedAt    time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// TableName 指定表名
func (MessageTemplate) TableName() string {
	return "message_templates"
}

// GetVariables 获取变量列表（反序列化）
func (m *MessageTemplate) GetVariables() ([]string, error) {
	var variables []string
	if m.Variables == "" {
		return variables, nil
	}
	err := json.Unmarshal([]byte(m.Variables), &variables)
	return variables, err
}

// SetVariables 设置变量列表（序列化）
func (m *MessageTemplate) SetVariables(variables []string) error {
	data, err := json.Marshal(variables)
	if err != nil {
		return err
	}
	m.Variables = string(data)
	return nil
}
