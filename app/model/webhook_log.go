package model

import (
	"time"
)

// WebhookLog Webhook 通知日志表
type WebhookLog struct {
	ID              uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID          string    `gorm:"type:varchar(36);index:idx_task_id;comment:任务ID" json:"task_id"`
	AppID           string    `gorm:"type:varchar(32);not null;index:idx_app_id;comment:应用ID" json:"app_id"`
	WebhookConfigID uint      `gorm:"type:bigint unsigned;index:idx_webhook_config;comment:Webhook配置ID" json:"webhook_config_id"`
	WebhookURL      string    `gorm:"type:varchar(500);not null;comment:Webhook地址" json:"webhook_url"`
	Event           string    `gorm:"type:varchar(20);not null;comment:事件类型" json:"event"`
	RequestData     string    `gorm:"type:json;comment:请求数据" json:"request_data"`
	ResponseStatus  int       `gorm:"type:int;comment:HTTP响应状态码" json:"response_status"`
	ResponseData    string    `gorm:"type:text;comment:响应内容" json:"response_data"`
	Status          string    `gorm:"type:varchar(20);not null;comment:状态: success/failed" json:"status"`
	ErrorMessage    string    `gorm:"type:text;comment:错误信息" json:"error_message"`
	RetryCount      int       `gorm:"type:int;default:0;comment:重试次数" json:"retry_count"`
	CreatedAt       time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;index:idx_created_at" json:"created_at"`
}

// TableName 指定表名
func (WebhookLog) TableName() string {
	return "webhook_logs"
}
