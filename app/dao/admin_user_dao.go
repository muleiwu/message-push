package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

type AdminUserDAO struct {
	db *gorm.DB
}

func NewAdminUserDAO() *AdminUserDAO {
	return &AdminUserDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// GetByUsername 根据用户名获取管理员
func (d *AdminUserDAO) GetByUsername(username string) (*model.AdminUser, error) {
	var user model.AdminUser
	err := d.db.Where("username = ? AND status = 1", username).First(&user).Error
	return &user, err
}

// GetByID 根据ID获取管理员
func (d *AdminUserDAO) GetByID(id uint) (*model.AdminUser, error) {
	var user model.AdminUser
	err := d.db.Where("id = ? AND status = 1", id).First(&user).Error
	return &user, err
}

// Create 创建管理员
func (d *AdminUserDAO) Create(user *model.AdminUser) error {
	return d.db.Create(user).Error
}

// Update 更新管理员
func (d *AdminUserDAO) Update(user *model.AdminUser) error {
	return d.db.Save(user).Error
}

// UsernameExists 检查用户名是否存在
func (d *AdminUserDAO) UsernameExists(username string) bool {
	var count int64
	d.db.Model(&model.AdminUser{}).Where("username = ?", username).Count(&count)
	return count > 0
}
