package dto

// CreateApplicationRequest 创建应用请求
type CreateApplicationRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=50"`
	Description string `json:"description" binding:"max=200"`
	Status      int    `json:"status" binding:"omitempty,oneof=1 2"` // 1:启用 2:禁用
}

// UpdateApplicationRequest 更新应用请求
type UpdateApplicationRequest struct {
	Name        string `json:"name" binding:"omitempty,min=2,max=50"`
	Description string `json:"description" binding:"omitempty,max=200"`
	Status      int    `json:"status" binding:"omitempty,oneof=1 2"`
}

// ApplicationListRequest 应用列表请求
type ApplicationListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Name     string `form:"name" binding:"omitempty,max=50"`
	Status   int    `form:"status" binding:"omitempty,oneof=1 2"`
}

// ApplicationResponse 应用响应
type ApplicationResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AppKey      string `json:"app_key"`
	AppSecret   string `json:"app_secret,omitempty"` // 仅创建时返回明文
	Status      int    `json:"status"`
	DailyLimit  int    `json:"daily_limit"`
	QPSLimit    int    `json:"qps_limit"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// ApplicationListResponse 应用列表响应
type ApplicationListResponse struct {
	Total int                    `json:"total"`
	Page  int                    `json:"page"`
	Size  int                    `json:"size"`
	Items []*ApplicationResponse `json:"items"`
}

// RegenerateSecretRequest 重新生成密钥请求
type RegenerateSecretRequest struct {
	AppID uint `json:"app_id" binding:"required"`
}

// RegenerateSecretResponse 重新生成密钥响应
type RegenerateSecretResponse struct {
	AppKey    string `json:"app_key"`
	AppSecret string `json:"app_secret"`
}

// CreateProviderRequest 创建服务商请求
type CreateProviderRequest struct {
	Name        string                 `json:"name" binding:"required,min=2,max=50"`
	Type        string                 `json:"type" binding:"required,oneof=sms email wechat_work dingtalk"`
	Description string                 `json:"description" binding:"max=200"`
	Config      map[string]interface{} `json:"config" binding:"required"`
	Status      int                    `json:"status" binding:"omitempty,oneof=1 2"`
}

// UpdateProviderRequest 更新服务商请求
type UpdateProviderRequest struct {
	Name        string                 `json:"name" binding:"omitempty,min=2,max=50"`
	Description string                 `json:"description" binding:"omitempty,max=200"`
	Config      map[string]interface{} `json:"config" binding:"omitempty"`
	Status      int                    `json:"status" binding:"omitempty,oneof=1 2"`
}

// ProviderListRequest 服务商列表请求
type ProviderListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Type     string `form:"type" binding:"omitempty,oneof=sms email wechat_work dingtalk"`
	Status   int    `form:"status" binding:"omitempty,oneof=1 2"`
}

// ProviderResponse 服务商响应
type ProviderResponse struct {
	ID          uint                   `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Config      map[string]interface{} `json:"config"`
	Status      int                    `json:"status"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// ProviderListResponse 服务商列表响应
type ProviderListResponse struct {
	Total int                 `json:"total"`
	Page  int                 `json:"page"`
	Size  int                 `json:"size"`
	Items []*ProviderResponse `json:"items"`
}

// CreateChannelRequest 创建通道请求
type CreateChannelRequest struct {
	Name   string `json:"name" binding:"required,min=2,max=50"`
	Type   string `json:"type" binding:"required,oneof=sms email wechat_work dingtalk"`
	Status int    `json:"status" binding:"omitempty,oneof=1 2"`
}

// UpdateChannelRequest 更新通道请求
type UpdateChannelRequest struct {
	Name   string `json:"name" binding:"omitempty,min=2,max=50"`
	Status int    `json:"status" binding:"omitempty,oneof=1 2"`
}

// ChannelListRequest 通道列表请求
type ChannelListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Type     string `form:"type" binding:"omitempty,oneof=sms email wechat_work dingtalk"`
	Status   int    `form:"status" binding:"omitempty,oneof=1 2"`
}

// ChannelResponse 通道响应
type ChannelResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Status    int    `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ChannelListResponse 通道列表响应
type ChannelListResponse struct {
	Total int                `json:"total"`
	Page  int                `json:"page"`
	Size  int                `json:"size"`
	Items []*ChannelResponse `json:"items"`
}

// BindProviderToChannelRequest 绑定服务商到通道
type BindProviderToChannelRequest struct {
	ChannelID  uint `json:"channel_id" binding:"required"`
	ProviderID uint `json:"provider_id" binding:"required"`
	Priority   int  `json:"priority" binding:"omitempty,min=0,max=100"`
	Weight     int  `json:"weight" binding:"omitempty,min=1,max=100"`
}

// StatisticsRequest 统计查询请求
type StatisticsRequest struct {
	StartDate string `form:"start_date" binding:"required"` // YYYY-MM-DD
	EndDate   string `form:"end_date" binding:"required"`   // YYYY-MM-DD
	AppID     uint   `form:"app_id" binding:"omitempty"`
	ChannelID uint   `form:"channel_id" binding:"omitempty"`
}

// DailyStatistics 每日统计
type DailyStatistics struct {
	Date         string `json:"date"`
	TotalCount   int64  `json:"total_count"`
	SuccessCount int64  `json:"success_count"`
	FailureCount int64  `json:"failure_count"`
	SuccessRate  string `json:"success_rate"`
}

// StatisticsResponse 统计响应
type StatisticsResponse struct {
	Summary struct {
		TotalCount   int64  `json:"total_count"`
		SuccessCount int64  `json:"success_count"`
		FailureCount int64  `json:"failure_count"`
		SuccessRate  string `json:"success_rate"`
	} `json:"summary"`
	Daily []*DailyStatistics `json:"daily"`
}
