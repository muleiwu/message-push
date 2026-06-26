package model

import (
	"time"
)

// CallbackLog 服务商回调日志表（统一记录下行投递回执与上行短信）
type CallbackLog struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Type           string    `gorm:"type:varchar(20);not null;default:report;index:idx_type;comment:回调类型: report=下行回执, upstream=上行短信" json:"type"`
	TaskID         string    `gorm:"type:varchar(36);index:idx_task_id;comment:任务ID" json:"task_id"`
	AppID          string    `gorm:"type:varchar(32);not null;index:idx_app_id;comment:应用ID" json:"app_id"`
	ProviderCode   string    `gorm:"type:varchar(32);not null;index:idx_provider_code;comment:服务商代码" json:"provider_code"`
	ProviderID     string    `gorm:"type:varchar(64);index:idx_provider_id;comment:服务商消息ID（上行为空）" json:"provider_id"`
	Mobile         string    `gorm:"type:varchar(20);index:idx_mobile;comment:手机号（回执=接收方, 上行=发送方）" json:"mobile"`
	Content        string    `gorm:"type:text;comment:上行短信回复内容" json:"content"`
	CallbackStatus string    `gorm:"type:varchar(20);comment:回调状态: delivered/failed/rejected（上行为空）" json:"callback_status"`
	ErrorCode      string    `gorm:"type:varchar(32);comment:错误码" json:"error_code"`
	ErrorMessage   string    `gorm:"type:text;comment:错误信息" json:"error_message"`
	RawData        string    `gorm:"type:json;comment:原始回调数据" json:"raw_data"`
	CreatedAt      time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;index:idx_created_at" json:"created_at"`
}

// TableName 指定表名
func (CallbackLog) TableName() string {
	return "callback_logs"
}
