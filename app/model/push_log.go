package model

import (
	"time"
)

// PushLog 推送日志表
type PushLog struct {
	ID                uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID            string    `gorm:"type:varchar(36);not null;index:idx_task_id;comment:任务ID" json:"task_id"`
	AppID             string    `gorm:"type:varchar(32);not null;index:idx_app_id_created;comment:应用ID" json:"app_id"`
	ProviderAccountID uint      `gorm:"type:bigint unsigned;not null;index:idx_provider_account;comment:服务商账号ID" json:"provider_account_id"`
	ProviderMsgID     string    `gorm:"type:varchar(100);index:idx_provider_msg_id;comment:服务商返回的消息ID" json:"provider_msg_id"`
	RequestData       string    `gorm:"type:json;comment:请求数据" json:"request_data"`
	ResponseData      string    `gorm:"type:json;comment:响应数据" json:"response_data"`
	Status            string    `gorm:"type:varchar(20);not null;index:idx_status_created;comment:状态：success, failed" json:"status"`
	ErrorMessage      string    `gorm:"type:text;comment:错误信息" json:"error_message"`
	CostTime          int       `gorm:"type:int;comment:耗时（毫秒）" json:"cost_time"`
	CreatedAt         time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;index:idx_created_at,idx_app_id_created,idx_status_created" json:"created_at"`
}

// TableName 指定表名
func (PushLog) TableName() string {
	return "push_logs"
}
