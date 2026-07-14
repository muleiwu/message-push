package server

import (
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	appHelper "cnb.cool/mliev/push/message-push/app/helper"
)

// JWTConfigServer 在启动时将 jwt.secret / jwt.expire_hours 配置接线到 JWT 助手。
//
// secret 未配置时保留内置默认值（多副本部署必须显式配置且各副本一致）。
type JWTConfigServer struct{}

// NewJWTConfigServer 创建 JWTConfigServer。
func NewJWTConfigServer() *JWTConfigServer {
	return &JWTConfigServer{}
}

// Run 读取配置并应用到 JWT 助手。
func (receiver *JWTConfigServer) Run() error {
	config := helper.GetConfig()

	if secret := config.GetString("jwt.secret", ""); secret != "" {
		appHelper.SetJWTSecret(secret)
	} else {
		helper.GetLogger().Warn("[jwt] 未配置 JWT_SECRET，使用内置默认密钥（生产环境请务必配置）")
	}

	if hours := config.GetInt("jwt.expire_hours", 24); hours > 0 {
		appHelper.SetJWTExpire(time.Duration(hours) * time.Hour)
	}
	return nil
}

// Stop 无需停止操作。
func (receiver *JWTConfigServer) Stop() error {
	return nil
}
