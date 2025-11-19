package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// ApplicationDAO 应用数据访问对象
type ApplicationDAO struct {
	db *gorm.DB
}

// NewApplicationDAO 创建ApplicationDAO
func NewApplicationDAO() *ApplicationDAO {
	return &ApplicationDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建应用
func (d *ApplicationDAO) Create(app *model.Application) error {
	return d.db.Create(app).Error
}

// GetByID 根据ID获取应用
func (d *ApplicationDAO) GetByID(id uint) (*model.Application, error) {
	var app model.Application
	err := d.db.Where("id = ?", id).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// GetByAppID 根据AppID获取应用
func (d *ApplicationDAO) GetByAppID(appID string) (*model.Application, error) {
	var app model.Application
	err := d.db.Where("app_id = ?", appID).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// Update 更新应用
func (d *ApplicationDAO) Update(app *model.Application) error {
	return d.db.Save(app).Error
}

// Delete 删除应用（软删除）
func (d *ApplicationDAO) Delete(id uint) error {
	return d.db.Delete(&model.Application{}, id).Error
}

// List 获取应用列表
func (d *ApplicationDAO) List(page, pageSize int) ([]*model.Application, int64, error) {
	var apps []*model.Application
	var total int64

	offset := (page - 1) * pageSize

	// 计算总数
	if err := d.db.Model(&model.Application{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询列表
	err := d.db.Offset(offset).Limit(pageSize).Order("id DESC").Find(&apps).Error
	if err != nil {
		return nil, 0, err
	}

	return apps, total, nil
}

// GetByStatus 根据状态获取应用列表
func (d *ApplicationDAO) GetByStatus(status int8) ([]*model.Application, error) {
	var apps []*model.Application
	err := d.db.Where("status = ?", status).Find(&apps).Error
	if err != nil {
		return nil, err
	}
	return apps, nil
}
