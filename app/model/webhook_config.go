package model

import (
	"time"
)

// WebhookConfig 应用的 Webhook 配置
type WebhookConfig struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	AppID       string    `gorm:"type:varchar(32);not null;uniqueIndex:uk_app_id;comment:应用ID" json:"app_id"`
	WebhookURL  string    `gorm:"type:varchar(500);not null;comment:回调地址" json:"webhook_url"`
	Secret      string    `gorm:"type:varchar(64);comment:签名密钥" json:"secret"`
	Events      string    `gorm:"type:varchar(200);default:'success,failed';comment:订阅事件：success,failed,delivered" json:"events"`
	Status      int       `gorm:"type:tinyint;default:1;comment:状态：0-禁用 1-启用" json:"status"`
	RetryCount  int       `gorm:"type:int;default:3;comment:最大重试次数" json:"retry_count"`
	Timeout     int       `gorm:"type:int;default:5;comment:超时时间（秒）" json:"timeout"`
	Description string    `gorm:"type:varchar(200);comment:描述" json:"description"`
	CreatedAt   time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
}

// TableName 指定表名
func (WebhookConfig) TableName() string {
	return "webhook_configs"
}

// IsEnabled 是否启用
func (w *WebhookConfig) IsEnabled() bool {
	return w.Status == 1
}

// ShouldNotify 是否应该通知某个事件
func (w *WebhookConfig) ShouldNotify(event string) bool {
	if !w.IsEnabled() {
		return false
	}

	// 检查事件是否在订阅列表中
	// events 格式：success,failed,delivered
	events := w.Events
	if events == "" {
		return false
	}

	// 简单的字符串包含检查
	return containsEvent(events, event)
}

// containsEvent 检查事件列表是否包含指定事件
func containsEvent(events, event string) bool {
	// 简单实现，可以优化为解析后的 map
	for i := 0; i < len(events); {
		j := i
		for j < len(events) && events[j] != ',' {
			j++
		}
		if events[i:j] == event {
			return true
		}
		i = j + 1
	}
	return false
}
