package dao

import (
	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"gorm.io/gorm"
)

type AdminUserDAO struct {
	db *gorm.DB
}

func NewAdminUserDAO() *AdminUserDAO {
	return &AdminUserDAO{
		db: helper.GetDatabase(),
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
	err := d.db.Where("id = ?", id).First(&user).Error
	return &user, err
}

// GetByOidcSub 根据 OIDC subject 获取管理员（不过滤 status，由调用方区分禁用与不存在）
func (d *AdminUserDAO) GetByOidcSub(sub string) (*model.AdminUser, error) {
	var user model.AdminUser
	err := d.db.Where("oidc_sub = ?", sub).First(&user).Error
	return &user, err
}

// GetByEmail 根据邮箱获取管理员（不过滤 status，由调用方区分禁用与不存在）
func (d *AdminUserDAO) GetByEmail(email string) (*model.AdminUser, error) {
	var user model.AdminUser
	err := d.db.Where("email = ?", email).First(&user).Error
	return &user, err
}

// BindOidcSub 仅在账号尚未绑定或已绑定相同 subject 时更新。
// 返回 false 表示账号已被其他 subject 抢先绑定，调用方必须拒绝覆盖。
func (d *AdminUserDAO) BindOidcSub(id uint, sub string) (bool, error) {
	result := d.db.Model(&model.AdminUser{}).
		Where("id = ? AND (oidc_sub IS NULL OR oidc_sub = ?)", id, sub).
		Update("oidc_sub", sub)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		return true, nil
	}

	// MySQL 默认可能把“值未变化”报告为 0 affected rows；相同 subject 仍视为成功。
	var user model.AdminUser
	if err := d.db.Select("oidc_sub").Where("id = ?", id).First(&user).Error; err != nil {
		return false, err
	}
	return user.OidcSub != nil && *user.OidcSub == sub, nil
}

// Create 创建管理员
func (d *AdminUserDAO) Create(user *model.AdminUser) error {
	return d.db.Create(user).Error
}

// Update 更新管理员
func (d *AdminUserDAO) Update(user *model.AdminUser) error {
	// 仅更新 CRUD 允许修改的字段，避免与 OIDC 绑定或密码重置并发时
	// Save 整行覆盖 oidc_sub/password 等身份字段。map 保证 status=0 也会写入。
	return d.db.Model(&model.AdminUser{}).Where("id = ?", user.ID).Updates(map[string]any{
		"username":  user.Username,
		"real_name": user.RealName,
		"email":     user.Email,
		"status":    user.Status,
	}).Error
}

// UsernameExists 检查用户名是否存在
func (d *AdminUserDAO) UsernameExists(username string) bool {
	var count int64
	d.db.Model(&model.AdminUser{}).Where("username = ?", username).Count(&count)
	return count > 0
}

// EmailExists 检查规范化邮箱是否已被其他管理员使用。
// 唯一索引也覆盖软删除记录，因此这里使用 Unscoped 与数据库语义保持一致。
func (d *AdminUserDAO) EmailExists(email string, excludeID uint) (bool, error) {
	var count int64
	query := d.db.Unscoped().Model(&model.AdminUser{}).Where("email = ?", email)
	if excludeID != 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// CountAll 统计所有管理员用户数量
func (d *AdminUserDAO) CountAll(count *int64) error {
	return d.db.Model(&model.AdminUser{}).Count(count).Error
}

// GetList 获取管理员用户列表
func (d *AdminUserDAO) GetList(page, pageSize int, username string, status *int8) ([]*model.AdminUser, int64, error) {
	var users []*model.AdminUser
	var total int64

	query := d.db.Model(&model.AdminUser{})

	// 条件过滤
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// Delete 删除管理员用户（软删除）
func (d *AdminUserDAO) Delete(id uint) error {
	return d.db.Delete(&model.AdminUser{}, id).Error
}

// UpdatePassword 更新密码
func (d *AdminUserDAO) UpdatePassword(id uint, hashedPassword string) error {
	return d.db.Model(&model.AdminUser{}).Where("id = ?", id).Update("password", hashedPassword).Error
}
