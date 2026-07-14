package infrastructure

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/identity/domain"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

// 确保实现 domain.OIDCService 端口
var _ domain.OIDCService = (*OIDCService)(nil)

// oidcStateKeyPrefix Redis 中 state→nonce 的键前缀
const oidcStateKeyPrefix = "oidc:state:"

// oidcStateTTL state/nonce 的有效期
const oidcStateTTL = 10 * time.Minute

// adminUserStore 抽象 JIT 开通所需的 DAO 操作，便于单测替换。
type adminUserStore interface {
	GetByOidcSub(sub string) (*model.AdminUser, error)
	GetByEmail(email string) (*model.AdminUser, error)
	BindOidcSub(id uint, sub string) error
	UsernameExists(username string) bool
	Create(user *model.AdminUser) error
}

// OIDCService 管理后台 OIDC SSO 登录服务。
//
// IdP 元数据（discovery）懒初始化并缓存：IdP 暂不可达不影响服务启动，
// 首次登录请求时再发现，失败后后续请求可重试。
type OIDCService struct {
	mu           sync.Mutex
	provider     *oidc.Provider
	cachedIssuer string

	newStore func() adminUserStore
}

// NewOIDCService 创建 OIDC 登录服务。
func NewOIDCService() *OIDCService {
	return &OIDCService{
		newStore: func() adminUserStore { return dao.NewAdminUserDAO() },
	}
}

// Enabled 返回 OIDC 登录是否启用且配置完整。
func (s *OIDCService) Enabled() bool {
	config := helper.GetConfig()
	return config.GetBool("oidc.enabled", false) &&
		config.GetString("oidc.issuer", "") != "" &&
		config.GetString("oidc.client_id", "") != "" &&
		config.GetString("oidc.client_secret", "") != "" &&
		config.GetString("oidc.redirect_url", "") != ""
}

// DisplayName 返回登录页 SSO 按钮的显示名称。
func (s *OIDCService) DisplayName() string {
	return helper.GetConfig().GetString("oidc.display_name", "SSO 登录")
}

// getProvider 懒初始化并缓存 OIDC provider（issuer 变更时重新发现）。
func (s *OIDCService) getProvider(ctx context.Context) (*oidc.Provider, error) {
	issuer := helper.GetConfig().GetString("oidc.issuer", "")
	if issuer == "" {
		return nil, errors.New("OIDC issuer 未配置")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.provider != nil && s.cachedIssuer == issuer {
		return s.provider, nil
	}

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("OIDC issuer 发现失败: %w", err)
	}
	s.provider = provider
	s.cachedIssuer = issuer
	return provider, nil
}

// oauth2Config 基于当前配置与 provider 端点构造 oauth2 配置。
func (s *OIDCService) oauth2Config(provider *oidc.Provider) *oauth2.Config {
	config := helper.GetConfig()
	return &oauth2.Config{
		ClientID:     config.GetString("oidc.client_id", ""),
		ClientSecret: config.GetString("oidc.client_secret", ""),
		RedirectURL:  config.GetString("oidc.redirect_url", ""),
		Endpoint:     provider.Endpoint(),
		Scopes:       strings.Fields(config.GetString("oidc.scopes", "openid profile email")),
	}
}

// BuildAuthURL 生成 IdP 授权地址，state→nonce 写入 Redis。
func (s *OIDCService) BuildAuthURL(ctx context.Context) (string, error) {
	if !s.Enabled() {
		return "", errors.New("OIDC 登录未启用")
	}

	provider, err := s.getProvider(ctx)
	if err != nil {
		return "", err
	}

	state, err := randomHex(16)
	if err != nil {
		return "", fmt.Errorf("生成 state 失败: %w", err)
	}
	nonce, err := randomHex(16)
	if err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}

	if err := helper.GetRedis().Set(ctx, oidcStateKeyPrefix+state, nonce, oidcStateTTL).Err(); err != nil {
		return "", fmt.Errorf("保存 state 失败: %w", err)
	}

	return s.oauth2Config(provider).AuthCodeURL(state, oidc.Nonce(nonce)), nil
}

// HandleCallback 处理 IdP 回调：校验 state、换码、验 ID Token 与 nonce，JIT 映射本地管理员。
func (s *OIDCService) HandleCallback(ctx context.Context, state, code string) (*model.AdminUser, error) {
	if !s.Enabled() {
		return nil, errors.New("OIDC 登录未启用")
	}
	if state == "" || code == "" {
		return nil, errors.New("缺少 state 或 code 参数")
	}

	// GetDel 保证 state 单次使用（防重放）
	nonce, err := helper.GetRedis().GetDel(ctx, oidcStateKeyPrefix+state).Result()
	if err != nil || nonce == "" {
		return nil, errors.New("state 无效或已过期，请重新发起登录")
	}

	provider, err := s.getProvider(ctx)
	if err != nil {
		return nil, err
	}

	token, err := s.oauth2Config(provider).Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("授权码换取 token 失败: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, errors.New("IdP 响应中缺少 id_token")
	}

	clientID := helper.GetConfig().GetString("oidc.client_id", "")
	idToken, err := provider.Verifier(&oidc.Config{ClientID: clientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("ID Token 验证失败: %w", err)
	}
	if idToken.Nonce != nonce {
		return nil, errors.New("nonce 校验失败")
	}

	var claims struct {
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("解析 ID Token claims 失败: %w", err)
	}

	user, err := provisionOIDCUser(s.newStore(), idToken.Subject, claims.Email, claims.Name, claims.PreferredUsername)
	if err != nil {
		return nil, err
	}
	helper.GetLogger().Info("[oidc] SSO 登录成功: " + user.Username)
	return user, nil
}

// provisionOIDCUser 按 oidc_sub → email → 新建 的顺序 JIT 映射本地管理员。
func provisionOIDCUser(store adminUserStore, sub, email, name, preferredUsername string) (*model.AdminUser, error) {
	if sub == "" {
		return nil, errors.New("ID Token 缺少 subject")
	}

	// 1. 精确匹配已绑定的 oidc_sub
	user, err := store.GetByOidcSub(sub)
	if err == nil {
		return checkUserEnabled(user)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 2. 按邮箱匹配已有账号并绑定 sub
	if email != "" {
		user, err = store.GetByEmail(email)
		if err == nil {
			if _, err := checkUserEnabled(user); err != nil {
				return nil, err
			}
			if err := store.BindOidcSub(user.ID, sub); err != nil {
				return nil, fmt.Errorf("绑定 OIDC 账号失败: %w", err)
			}
			user.OidcSub = &sub
			return user, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("查询用户失败: %w", err)
		}
	}

	// 3. JIT 新建账号（密码置为随机值，仅可通过 SSO 登录）
	username := deriveUsername(store, preferredUsername, email, sub)
	randomPwd, err := randomHex(16)
	if err != nil {
		return nil, fmt.Errorf("生成随机密码失败: %w", err)
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(randomPwd), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	realName := name
	if realName == "" {
		realName = username
	}
	user = &model.AdminUser{
		Username:   username,
		Password:   string(hashedPassword),
		RealName:   realName,
		OidcSub:    &sub,
		AuthSource: "oidc",
		Status:     1,
	}
	if email != "" {
		user.Email = &email
	}
	if err := store.Create(user); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}
	return user, nil
}

// checkUserEnabled 校验账号未被禁用。
func checkUserEnabled(user *model.AdminUser) (*model.AdminUser, error) {
	if user.Status != 1 {
		return nil, domain.ErrUserDisabled
	}
	return user, nil
}

// deriveUsername 推导 JIT 账号用户名：preferred_username → 邮箱局部名 → oidc_ 前缀，
// 冲突时追加 _1、_2 … 后缀。
func deriveUsername(store adminUserStore, preferredUsername, email, sub string) string {
	base := preferredUsername
	if base == "" && email != "" {
		base = strings.SplitN(email, "@", 2)[0]
	}
	if base == "" {
		if len(sub) > 8 {
			sub = sub[:8]
		}
		base = "oidc_" + sub
	}
	// 用户名列为 varchar(50)，预留后缀空间
	if len(base) > 40 {
		base = base[:40]
	}

	username := base
	for i := 1; store.UsernameExists(username); i++ {
		username = fmt.Sprintf("%s_%d", base, i)
	}
	return username
}

// randomHex 生成 length 字节的随机十六进制串。
func randomHex(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
