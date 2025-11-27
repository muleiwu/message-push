package model

import (
	"time"
)

// CallbackLog 服务商回调日志表
type CallbackLog struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID         string    `gorm:"type:varchar(36);index:idx_task_id;comment:任务ID" json:"task_id"`
	AppID          string    `gorm:"type:varchar(32);not null;index:idx_app_id;comment:应用ID" json:"app_id"`
	ProviderCode   string    `gorm:"type:varchar(32);not null;index:idx_provider_code;comment:服务商代码" json:"provider_code"`
	ProviderID     string    `gorm:"type:varchar(64);index:idx_provider_id;comment:服务商消息ID" json:"provider_id"`
	CallbackStatus string    `gorm:"type:varchar(20);not null;comment:回调状态: delivered/failed/rejected" json:"callback_status"`
	ErrorCode      string    `gorm:"type:varchar(32);comment:错误码" json:"error_code"`
	ErrorMessage   string    `gorm:"type:text;comment:错误信息" json:"error_message"`
	RawData        string    `gorm:"type:json;comment:原始回调数据" json:"raw_data"`
	CreatedAt      time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;index:idx_created_at" json:"created_at"`
}

// TableName 指定表名
func (CallbackLog) TableName() string {
	return "callback_logs"
}
