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

// CreateAdminUserRequest 创建管理员用户请求
type CreateAdminUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6,max=32"`
	RealName string `json:"real_name" binding:"required,min=2,max=100"`
	Status   int8   `json:"status" binding:"omitempty,oneof=0 1"` // 1:启用 0:禁用
}

// UpdateAdminUserRequest 更新管理员用户请求
type UpdateAdminUserRequest struct {
	RealName string `json:"real_name" binding:"omitempty,min=2,max=100"`
	Status   int8   `json:"status" binding:"omitempty,oneof=0 1"`
}

// AdminUserListRequest 管理员用户列表请求
type AdminUserListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Username string `form:"username" binding:"omitempty,max=50"`
	Status   *int8  `form:"status" binding:"omitempty,oneof=0 1"`
}

// AdminUserResponse 管理员用户响应
type AdminUserResponse struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	RealName  string `json:"real_name"`
	Status    int8   `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// AdminUserListResponse 管理员用户列表响应
type AdminUserListResponse struct {
	Total int                  `json:"total"`
	Page  int                  `json:"page"`
	Size  int                  `json:"size"`
	Items []*AdminUserResponse `json:"items"`
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	Password     string `json:"password" binding:"omitempty,min=6,max=32"` // 为空则随机生成
	AutoGenerate bool   `json:"auto_generate"`                             // true:自动生成随机密码
}

// ResetPasswordResponse 重置密码响应
type ResetPasswordResponse struct {
	Password string `json:"password"` // 仅在重置时返回明文密码
}
