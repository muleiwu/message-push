package service

import (
	"crypto/rand"
	"encoding/hex"
)

// AdminService 管理后台服务
type AdminService struct{}

// NewAdminService 创建管理后台服务实例
func NewAdminService() *AdminService {
	return &AdminService{}
}

// generateRandomKey 生成随机密钥
func generateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
