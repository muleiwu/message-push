package autoload

import "cnb.cool/mliev/open/go-web/pkg/helper"

// Jwt 管理后台 JWT 签发配置。
//
// jwt.secret → JWT_SECRET，jwt.expire_hours → JWT_EXPIRE_HOURS。
// secret 为空时沿用内置默认值（仅适合单机试用）；多副本部署必须显式设置且各副本一致。
type Jwt struct {
}

func (receiver Jwt) InitConfig() map[string]any {
	env := helper.GetEnv()
	return map[string]any{
		"jwt.secret":       env.GetString("jwt.secret", ""),
		"jwt.expire_hours": env.GetInt("jwt.expire_hours", 24),
	}
}
