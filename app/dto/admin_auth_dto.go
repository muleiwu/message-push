package dto

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	AccessToken string `json:"accessToken"`
}

// UserInfoResponse 用户信息响应
type UserInfoResponse struct {
	UserID   uint     `json:"userId"`
	Username string   `json:"username"`
	RealName string   `json:"realName"`
	Roles    []string `json:"roles"`
	HomePath string   `json:"homePath,omitempty"`
}

// AccessCodesResponse 权限码响应
type AccessCodesResponse []string
