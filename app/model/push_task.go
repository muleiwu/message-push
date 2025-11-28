package model

import (
	"time"
)

// PushTask 推送任务表
type PushTask struct {
	ID             uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID         string     `gorm:"type:varchar(36);uniqueIndex:uk_task_id;not null;comment:任务UUID" json:"task_id"`
	AppID          string     `gorm:"type:varchar(32);not null;index:idx_app_id_status;comment:应用ID" json:"app_id"`
	ChannelID      uint       `gorm:"type:bigint unsigned;not null;index:idx_channel;comment:通道ID" json:"channel_id"`
	MessageType    string     `gorm:"type:varchar(20);not null;comment:消息类型：sms, email等" json:"message_type"`
	Receiver       string     `gorm:"type:varchar(100);not null;comment:接收者（手机号/邮箱/UserID等）" json:"receiver"`
	Title          string     `gorm:"type:varchar(200);comment:标题（邮件、企微、钉钉使用）" json:"title"`
	Content        string     `gorm:"type:text;comment:内容（直接发送或模板渲染后内容）" json:"content"`
	TemplateCode   string     `gorm:"type:varchar(50);comment:模板代码" json:"template_code"`
	TemplateParams string     `gorm:"type:json;comment:模板参数" json:"template_params"`
	Signature      string     `gorm:"type:varchar(50);comment:签名" json:"signature"`
	Status         string     `gorm:"type:varchar(20);default:'pending';index:idx_app_id_status,idx_status_scheduled;comment:状态：pending, processing, success, failed" json:"status"`
	CallbackStatus string     `gorm:"type:varchar(20);comment:回调状态：pending, delivered, failed, rejected" json:"callback_status"`
	CallbackTime   *time.Time `gorm:"type:timestamp;comment:回调时间" json:"callback_time"`
	RetryCount     int        `gorm:"type:int;default:0;comment:已重试次数" json:"retry_count"`
	MaxRetry       int        `gorm:"type:int;default:3;comment:最大重试次数" json:"max_retry"`
	ScheduledAt    *time.Time `gorm:"type:timestamp;index:idx_status_scheduled;comment:定时发送时间" json:"scheduled_at"`
	CreatedAt      time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;index:idx_created_at" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	Channel        *Channel   `gorm:"foreignKey:ChannelID;references:ID" json:"channel,omitempty"`
}

// TableName 指定表名
func (PushTask) TableName() string {
	return "push_tasks"
}
