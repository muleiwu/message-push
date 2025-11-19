package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// CreateApp 创建应用
func CreateApp(app *model.Application) error {
	db := helper.GetHelper().GetDatabase()
	return db.Create(app).Error
}

// GetAppByID 根据ID获取应用
func GetAppByID(id uint) (*model.Application, error) {
	var app model.Application
	db := helper.GetHelper().GetDatabase()
	err := db.Where("id = ?", id).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// GetAppByAppID 根据AppID获取应用
func GetAppByAppID(appID string) (*model.Application, error) {
	var app model.Application
	db := helper.GetHelper().GetDatabase()
	err := db.Where("app_id = ?", appID).First(&app).Error
	if err != nil {
		return nil, err
	}
	return &app, nil
}

// UpdateApp 更新应用
func UpdateApp(id uint, updates map[string]interface{}) error {
	db := helper.GetHelper().GetDatabase()
	return db.Model(&model.Application{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteApp 删除应用（软删除）
func DeleteApp(id uint) error {
	db := helper.GetHelper().GetDatabase()
	return db.Delete(&model.Application{}, id).Error
}
