package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ParamMappingType 参数映射类型
type ParamMappingType string

const (
	ParamMappingTypeFixed   ParamMappingType = "fixed"   // 固定值
	ParamMappingTypeMapping ParamMappingType = "mapping" // 映射系统变量
)

// ParamMappingItem 参数映射项
type ParamMappingItem struct {
	Type        ParamMappingType `json:"type"`         // 映射类型：fixed=固定值, mapping=映射系统变量
	ProviderVar string           `json:"provider_var"` // 供应商模板变量名
	SystemVar   string           `json:"system_var"`   // 系统变量名（type=mapping时使用）
	Value       string           `json:"value"`        // 固定值（type=fixed时使用）
}

// ChannelTemplateBinding 通道模板绑定配置表
type ChannelTemplateBinding struct {
	ID                   uint              `gorm:"primaryKey;autoIncrement" json:"id"`
	ChannelID            uint              `gorm:"type:bigint unsigned;not null;index:idx_channel;comment:通道ID（关联channels表）" json:"channel_id"`
	ProviderTemplateID   uint              `gorm:"type:bigint unsigned;not null;comment:供应商模板ID（关联provider_templates表）" json:"provider_template_id"`
	ProviderID           uint              `gorm:"type:bigint unsigned;not null;index:idx_provider;comment:供应商账号ID（冗余字段，便于查询）" json:"provider_id"`
	ParamMapping         string            `gorm:"type:json;comment:参数映射，JSON数组格式 [{type,provider_var,system_var,value}]" json:"param_mapping"`
	Weight               int               `gorm:"type:int;default:10;comment:权重（同优先级下按权重分配流量）" json:"weight"`
	Priority             int               `gorm:"type:int;default:100;comment:优先级（数字越小越优先）" json:"priority"`
	Status               int8              `gorm:"type:tinyint;default:1;comment:状态：1=启用 0=禁用" json:"status"`
	IsActive             int8              `gorm:"type:tinyint;default:1;comment:是否激活：1=是 0=否" json:"is_active"`
	AutoDisableOnFail    bool              `gorm:"type:tinyint;default:0;comment:失败时自动禁用" json:"auto_disable_on_fail"`
	AutoDisableThreshold int               `gorm:"type:int;default:5;comment:自动禁用阈值（连续失败次数）" json:"auto_disable_threshold"`
	CreatedAt            time.Time         `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt            time.Time         `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt            gorm.DeletedAt    `gorm:"index" json:"deleted_at"`
	Channel              *Channel          `gorm:"foreignKey:ChannelID;references:ID" json:"channel,omitempty"`
	ProviderTemplate     *ProviderTemplate `gorm:"foreignKey:ProviderTemplateID;references:ID" json:"provider_template,omitempty"`
	ProviderAccount      *ProviderAccount  `gorm:"foreignKey:ProviderID;references:ID" json:"provider_account,omitempty"`
}

// TableName 指定表名
func (ChannelTemplateBinding) TableName() string {
	return "channel_template_bindings"
}

// GetParamMapping 获取参数映射（反序列化）
func (c *ChannelTemplateBinding) GetParamMapping() ([]ParamMappingItem, error) {
	var mapping []ParamMappingItem
	if c.ParamMapping == "" {
		return mapping, nil
	}
	err := json.Unmarshal([]byte(c.ParamMapping), &mapping)
	return mapping, err
}

// SetParamMapping 设置参数映射（序列化）
func (c *ChannelTemplateBinding) SetParamMapping(mapping []ParamMappingItem) error {
	data, err := json.Marshal(mapping)
	if err != nil {
		return err
	}
	c.ParamMapping = string(data)
	return nil
}
