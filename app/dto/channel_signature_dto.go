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
