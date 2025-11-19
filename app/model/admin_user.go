package model

import (
	"time"

	"gorm.io/gorm"
)

// AdminUser 管理员用户表
type AdminUser struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Username  string         `gorm:"type:varchar(50);uniqueIndex:uk_username;not null;comment:登录用户名" json:"username"`
	Password  string         `gorm:"type:varchar(255);not null;comment:密码(bcrypt加密)" json:"-"`
	RealName  string         `gorm:"type:varchar(100);not null;comment:真实姓名" json:"real_name"`
	Status    int8           `gorm:"type:tinyint;default:1;index:idx_status;comment:状态：1=启用 0=禁用" json:"status"`
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// TableName 指定表名
func (AdminUser) TableName() string {
	return "admin_users"
}
