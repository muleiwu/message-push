package autoload

import "cnb.cool/mliev/open/go-web/pkg/helper"

// Oidc 管理后台 OIDC SSO 登录配置。
//
// 环境变量映射（viper AutomaticEnv，点换下划线全大写）：
// oidc.enabled → OIDC_ENABLED，oidc.client_id → OIDC_CLIENT_ID，以此类推。
type Oidc struct {
}

func (receiver Oidc) InitConfig() map[string]any {
	env := helper.GetEnv()
	return map[string]any{
		"oidc.enabled":       env.GetBool("oidc.enabled", false),
		"oidc.issuer":        env.GetString("oidc.issuer", ""),
		"oidc.client_id":     env.GetString("oidc.client_id", ""),
		"oidc.client_secret": env.GetString("oidc.client_secret", ""),
		// redirect_url 必须是浏览器可达的绝对地址，指向后端回调，
		// 例如 https://push.example.com/api/admin/auth/oidc/callback
		"oidc.redirect_url": env.GetString("oidc.redirect_url", ""),
		"oidc.scopes":       env.GetString("oidc.scopes", "openid profile email"),
		"oidc.display_name": env.GetString("oidc.display_name", "SSO 登录"),
		// 回调成功后跳转的前端落地路径（或绝对地址），带 ?code= 一次性码
		"oidc.frontend_callback": env.GetString("oidc.frontend_callback", "/auth/sso-callback"),
		// 禁用用户名密码登录，仅允许 OIDC SSO 登录；
		// 仅在 OIDC 配置完整可用时生效，避免误配置导致无法登录
		"oidc.disable_password_login": env.GetBool("oidc.disable_password_login", false),
	}
}
