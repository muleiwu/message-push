package dao

import (
	"cnb.cool/mliev/examples/go-web/app/model"
	"cnb.cool/mliev/examples/go-web/internal/interfaces"
)

// TestDemoDao 注入
type TestDemoDao struct {
	helper interfaces.HelperInterface
}

func (receiver *TestDemoDao) GetUserByUsername(username string) (*model.TestDemo, error) {
	var user model.TestDemo
	if err := receiver.helper.GetDatabase().Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
