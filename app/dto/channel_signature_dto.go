package dto

// CreateChannelSignatureMappingRequest 创建通道签名映射请求
type CreateChannelSignatureMappingRequest struct {
	SignatureName       string `json:"signature_name" binding:"required"`
	ProviderSignatureID uint   `json:"provider_signature_id" binding:"required"`
	ProviderID          uint   `json:"provider_id" binding:"required"`
	Status              *int8  `json:"status"`
}

// UpdateChannelSignatureMappingRequest 更新通道签名映射请求
type UpdateChannelSignatureMappingRequest struct {
	SignatureName       string `json:"signature_name"`
	ProviderSignatureID uint   `json:"provider_signature_id"`
	Status              *int8  `json:"status"`
}

// ChannelSignatureMappingResponse 通道签名映射响应
type ChannelSignatureMappingResponse struct {
	ID                    uint   `json:"id"`
	ChannelID             uint   `json:"channel_id"`
	SignatureName         string `json:"signature_name"`
	ProviderSignatureID   uint   `json:"provider_signature_id"`
	ProviderSignatureName string `json:"provider_signature_name"`
	ProviderSignatureCode string `json:"provider_signature_code"`
	ProviderID            uint   `json:"provider_id"`
	ProviderAccountName   string `json:"provider_account_name"`
	ProviderCode          string `json:"provider_code"`
	Status                int8   `json:"status"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

// TestChannelRequest 测试通道发送请求
type TestChannelRequest struct {
	Receiver       string            `json:"receiver" binding:"required"`        // 接收者（手机号/邮箱等）
	SignatureName  string            `json:"signature_name"`                     // 签名名称（可选）
	TemplateParams map[string]string `json:"template_params" binding:"required"` // 模板参数
}

// TestChannelResponse 测试通道发送响应
type TestChannelResponse struct {
	Success bool   `json:"success"`           // 是否成功
	Message string `json:"message"`           // 消息
	TaskID  string `json:"task_id,omitempty"` // 任务ID（成功时返回）
}
