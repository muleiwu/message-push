package admin

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"time"

	gowebHelper "cnb.cool/mliev/open/go-web/pkg/helper"
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"

	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/modules/identity"
)

// oidcTicketKeyPrefix Redis 中一次性码→JWT 的键前缀
const oidcTicketKeyPrefix = "oidc:ticket:"

// oidcTicketTTL 一次性码有效期（仅用于回调后 SPA 立即换取 token）
const oidcTicketTTL = 60 * time.Second

// OIDCController 管理后台 OIDC SSO 登录控制器
type OIDCController struct {
}

// GetStatus 返回 OIDC 登录启用状态（登录页据此决定是否展示 SSO 按钮）
func (c OIDCController) GetStatus(ctx httpInterfaces.RouterContextInterface) {
	svc := identity.GetOIDCService()
	controller.SuccessResponse(ctx, dto.OIDCStatusResponse{
		Enabled:               svc.Enabled(),
		PasswordLoginDisabled: svc.PasswordLoginDisabled(),
		DisplayName:           svc.DisplayName(),
	})
}

// Authorize 生成 IdP 授权地址，前端拿到后自行跳转
func (c OIDCController) Authorize(ctx httpInterfaces.RouterContextInterface) {
	svc := identity.GetOIDCService()
	if !svc.Enabled() {
		controller.ErrorResponse(ctx, 400, "OIDC 登录未启用")
		return
	}

	authURL, err := svc.BuildAuthURL(ctx.Request().Context())
	if err != nil {
		gowebHelper.GetLogger().Error("[oidc] 生成授权地址失败: " + err.Error())
		controller.ErrorResponse(ctx, 500, "生成授权地址失败")
		return
	}
	controller.SuccessResponse(ctx, dto.OIDCAuthorizeResponse{URL: authURL})
}

// Callback 处理 IdP 授权回调：验证并 JIT 映射管理员后签发本地 JWT，
// 以一次性码经 302 交给前端落地页，避免 JWT 直接出现在 URL 中。
func (c OIDCController) Callback(ctx httpInterfaces.RouterContextInterface) {
	// IdP 主动返回错误（用户取消授权等）
	if errCode := ctx.Query("error"); errCode != "" {
		desc := ctx.Query("error_description")
		if desc == "" {
			desc = errCode
		}
		c.redirectFrontend(ctx, url.Values{"error": {desc}})
		return
	}

	svc := identity.GetOIDCService()
	user, err := svc.HandleCallback(ctx.Request().Context(), ctx.Query("state"), ctx.Query("code"))
	if err != nil {
		gowebHelper.GetLogger().Warn("[oidc] 回调处理失败: " + err.Error())
		msg := "SSO 登录失败，请重试"
		if errors.Is(err, identity.ErrUserDisabled) {
			msg = "账号已被禁用"
		}
		c.redirectFrontend(ctx, url.Values{"error": {msg}})
		return
	}

	token, err := helper.GenerateToken(user.ID, user.Username)
	if err != nil {
		gowebHelper.GetLogger().Error("[oidc] 生成令牌失败: " + err.Error())
		c.redirectFrontend(ctx, url.Values{"error": {"生成令牌失败"}})
		return
	}

	ticket, err := randomTicket()
	if err != nil {
		gowebHelper.GetLogger().Error("[oidc] 生成一次性码失败: " + err.Error())
		c.redirectFrontend(ctx, url.Values{"error": {"生成一次性码失败"}})
		return
	}
	redisCtx := ctx.Request().Context()
	if err := gowebHelper.GetRedis().Set(redisCtx, oidcTicketKeyPrefix+ticket, token, oidcTicketTTL).Err(); err != nil {
		gowebHelper.GetLogger().Error("[oidc] 保存一次性码失败: " + err.Error())
		c.redirectFrontend(ctx, url.Values{"error": {"登录会话保存失败"}})
		return
	}

	c.redirectFrontend(ctx, url.Values{"code": {ticket}})
}

// Exchange 用一次性码换取 accessToken（GETDEL 保证单次使用）
func (c OIDCController) Exchange(ctx httpInterfaces.RouterContextInterface) {
	var req dto.OIDCExchangeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "code 不能为空")
		return
	}

	token, err := gowebHelper.GetRedis().GetDel(ctx.Request().Context(), oidcTicketKeyPrefix+req.Code).Result()
	if err != nil || token == "" {
		controller.ErrorResponse(ctx, 401, "一次性码无效或已过期，请重新登录")
		return
	}

	controller.SuccessResponse(ctx, dto.LoginResponse{AccessToken: token})
}

// redirectFrontend 302 跳转到前端 SSO 落地页并附带查询参数
func (c OIDCController) redirectFrontend(ctx httpInterfaces.RouterContextInterface, params url.Values) {
	base := gowebHelper.GetConfig().GetString("oidc.frontend_callback", "/auth/sso-callback")
	sep := "?"
	if u, err := url.Parse(base); err == nil && u.RawQuery != "" {
		sep = "&"
	}
	ctx.Redirect(http.StatusFound, base+sep+params.Encode())
}

// randomTicket 生成 32 字符十六进制一次性码
func randomTicket() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
