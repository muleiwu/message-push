package domain

import (
	"context"
	"errors"

	"cnb.cool/mliev/push/message-push/app/model"
)

var (
	// ErrUserDisabled 表示 OIDC 对应的本地账号已被禁用。
	ErrUserDisabled = errors.New("账号已被禁用")
	// ErrOIDCIdentityConflict 表示邮箱对应账号已绑定另一个 OIDC subject。
	ErrOIDCIdentityConflict = errors.New("邮箱已绑定其他 SSO 身份")
)

// OIDCService 管理后台 OIDC SSO 登录端口。
//
// 授权码流程：BuildAuthURL 生成跳转 IdP 的授权地址（state/nonce 存 Redis），
// HandleCallback 校验回调、换取并验证 ID Token，按 JIT 策略映射/创建本地管理员。
type OIDCService interface {
	// Enabled 返回 OIDC 登录是否启用（oidc.enabled 且 issuer/client_id/redirect_url 完整）。
	Enabled() bool
	// DisplayName 返回登录页 SSO 按钮的显示名称。
	DisplayName() string
	// PasswordLoginDisabled 返回是否禁用密码登录（仅当 OIDC 可用时才会为 true，
	// 避免误配置导致完全无法登录）。
	PasswordLoginDisabled() bool
	// BuildAuthURL 生成 IdP 授权地址，state 与 nonce 写入 Redis（10 分钟有效）。
	BuildAuthURL(ctx context.Context) (string, error)
	// HandleCallback 处理 IdP 回调：校验 state、换码、验 ID Token 与 nonce，
	// 按 oidc_sub → email → 新建 的顺序 JIT 映射本地管理员。禁用账号返回错误。
	HandleCallback(ctx context.Context, state, code string) (*model.AdminUser, error)
}
