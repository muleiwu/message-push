package model

import (
	"time"
)

// WebhookLog Webhook 通知日志表
type WebhookLog struct {
	ID              uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID          string     `gorm:"type:varchar(36);index:idx_task_id;comment:任务ID" json:"task_id"`
	AppID           string     `gorm:"type:varchar(32);not null;index:idx_app_id;comment:应用ID" json:"app_id"`
	WebhookConfigID uint       `gorm:"index:idx_webhook_config;comment:Webhook配置ID" json:"webhook_config_id"`
	WebhookURL      string     `gorm:"type:varchar(500);not null;comment:Webhook地址" json:"webhook_url"`
	Event           string     `gorm:"type:varchar(20);not null;comment:事件类型" json:"event"`
	RequestData     string     `gorm:"type:json;comment:请求数据" json:"request_data"`
	ResponseStatus  int        `gorm:"type:int;comment:HTTP响应状态码" json:"response_status"`
	ResponseData    string     `gorm:"type:text;comment:响应内容" json:"response_data"`
	Status          string     `gorm:"type:varchar(20);not null;index:idx_webhook_logs_dispatch,priority:1;comment:状态: pending/processing/success/failed" json:"status"`
	ErrorMessage    string     `gorm:"type:text;comment:错误信息" json:"error_message"`
	RetryCount      int        `gorm:"type:int;default:0;comment:已执行的重试次数" json:"retry_count"`
	DedupKey        string     `gorm:"type:varchar(128);default:null;uniqueIndex:uk_webhook_logs_dedup_key;comment:事件幂等键" json:"dedup_key"`
	SigningSecret   string     `gorm:"type:varchar(64);comment:签名密钥快照" json:"-"`
	MaxRetries      int        `gorm:"type:int;default:3;comment:最大重试次数" json:"max_retries"`
	TimeoutSeconds  int        `gorm:"type:int;default:5;comment:单次请求超时秒数" json:"timeout_seconds"`
	NextAttemptAt   *time.Time `gorm:"index:idx_webhook_logs_dispatch,priority:2;comment:下次尝试时间" json:"next_attempt_at"`
	LockedUntil     *time.Time `gorm:"comment:投递租约到期时间" json:"locked_until"`
	LeaseToken      string     `gorm:"type:varchar(64);comment:投递租约令牌" json:"-"`
	CreatedAt       time.Time  `gorm:"default:CURRENT_TIMESTAMP;index:idx_created_at" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// TableName 指定表名
func (WebhookLog) TableName() string {
	return "webhook_logs"
}
