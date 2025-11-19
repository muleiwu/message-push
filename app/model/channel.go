package model

import (
	"time"

	"gorm.io/gorm"
)

// Channel 推送通道表
type Channel struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string         `gorm:"type:varchar(100);not null" json:"name"`
	Type      string         `gorm:"type:varchar(20);not null;index:idx_type;comment:类型：sms, email, wechat_work, dingtalk" json:"type"`
	Status    int8           `gorm:"type:tinyint;default:1;index:idx_status;comment:状态：1=启用 0=禁用" json:"status"`
	CreatedAt time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// TableName 指定表名
func (Channel) TableName() string {
	return "channels"
}
