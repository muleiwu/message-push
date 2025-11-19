package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// Provider 服务商表
type Provider struct {
	ID           uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	ProviderCode string         `gorm:"type:varchar(50);uniqueIndex:uk_provider_code;not null;comment:服务商代码：aliyun_sms, tencent_sms, smtp等" json:"provider_code"`
	ProviderName string         `gorm:"type:varchar(100);not null" json:"provider_name"`
	ProviderType string         `gorm:"type:varchar(20);not null;index:idx_type_status;comment:类型：sms, email, wechat_work, dingtalk" json:"provider_type"`
	Config       string         `gorm:"type:json;not null;comment:服务商配置（API Key、Secret等）" json:"config"`
	Status       int8           `gorm:"type:tinyint;default:1;index:idx_type_status;comment:状态：1=启用 0=禁用" json:"status"`
	Remark       string         `gorm:"type:text;comment:备注说明" json:"remark"`
	CreatedAt    time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// TableName 指定表名
func (Provider) TableName() string {
	return "providers"
}

// GetConfig 获取配置（反序列化）
func (p *Provider) GetConfig() (map[string]interface{}, error) {
	var config map[string]interface{}
	if p.Config == "" {
		return config, nil
	}
	err := json.Unmarshal([]byte(p.Config), &config)
	return config, err
}

// SetConfig 设置配置（序列化）
func (p *Provider) SetConfig(config map[string]interface{}) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	p.Config = string(data)
	return nil
}
