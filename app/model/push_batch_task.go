package model

import (
	"time"
)

// PushBatchTask 批量任务表
type PushBatchTask struct {
	ID           uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	BatchID      string    `gorm:"type:varchar(36);uniqueIndex:uk_batch_id;not null;comment:批次UUID" json:"batch_id"`
	AppID        string    `gorm:"type:varchar(32);not null;index:idx_app_id_status;comment:应用ID" json:"app_id"`
	TotalCount   int       `gorm:"type:int;default:0;comment:总任务数" json:"total_count"`
	SuccessCount int       `gorm:"type:int;default:0;comment:成功数" json:"success_count"`
	FailedCount  int       `gorm:"type:int;default:0;comment:失败数" json:"failed_count"`
	PendingCount int       `gorm:"type:int;default:0;comment:待处理数" json:"pending_count"`
	Status       string    `gorm:"type:varchar(20);default:'processing';index:idx_app_id_status;comment:状态：processing, completed, failed" json:"status"`
	CreatedAt    time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;index:idx_created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
}

// TableName 指定表名
func (PushBatchTask) TableName() string {
	return "push_batch_tasks"
}
