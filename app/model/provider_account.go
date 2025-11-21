package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ProviderAccount 服务商账号配置表
type ProviderAccount struct {
	ID           uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	AccountCode  string         `gorm:"type:varchar(50);uniqueIndex:uk_account_code;not null;comment:账号代码（唯一标识）" json:"account_code"`
	AccountName  string         `gorm:"type:varchar(100);not null" json:"account_name"`
	ProviderCode string         `gorm:"type:varchar(50);not null;index:idx_provider_type;comment:服务商代码：aliyun_sms, tencent_sms, smtp等" json:"provider_code"`
	ProviderType string         `gorm:"type:varchar(20);not null;index:idx_provider_type;comment:消息类型：sms, email, wechat_work, dingtalk, webhook, push" json:"provider_type"`
	Config       string         `gorm:"type:json;not null;comment:服务商配置（API Key、Secret等）" json:"config"`
	Status       int8           `gorm:"type:tinyint;default:1;index:idx_status;comment:状态：1=启用 0=禁用" json:"status"`
	Remark       string         `gorm:"type:text;comment:备注说明" json:"remark"`
	CreatedAt    time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// TableName 指定表名
func (ProviderAccount) TableName() string {
	return "provider_accounts"
}

// GetConfig 获取配置（反序列化）
func (p *ProviderAccount) GetConfig() (map[string]interface{}, error) {
	var config map[string]interface{}
	if p.Config == "" {
		return config, nil
	}
	err := json.Unmarshal([]byte(p.Config), &config)
	return config, err
}

// SetConfig 设置配置（序列化）
func (p *ProviderAccount) SetConfig(config map[string]interface{}) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	p.Config = string(data)
	return nil
}
