package dto

// CreateProviderSignatureRequest 创建供应商映射资源请求
type CreateProviderSignatureRequest struct {
	// SignatureCode 映射值：实际发送用；短信为供应商平台审核通过的签名，邮件为静态标题
	SignatureCode string `json:"signature_code" binding:"required,max=200" example:"墨蕾科技"`
	// SignatureName 资源名称：仅后台展示和识别，不参与实际发送
	SignatureName string `json:"signature_name" binding:"required,max=100" example:"墨蕾科技验证码签名"`
	Status        int8   `json:"status" example:"1"`
	Remark        string `json:"remark" example:"用于发送验证码"`
}

// UpdateProviderSignatureRequest 更新供应商映射资源请求
type UpdateProviderSignatureRequest struct {
	// SignatureCode 映射值：实际发送用；短信为供应商平台审核通过的签名，邮件为静态标题
	SignatureCode string `json:"signature_code" binding:"required,max=200" example:"墨蕾科技"`
	// SignatureName 资源名称：仅后台展示和识别，不参与实际发送
	SignatureName string `json:"signature_name" binding:"required,max=100" example:"墨蕾科技验证码签名"`
	Status        int8   `json:"status" example:"1"`
	Remark        string `json:"remark" example:"用于发送验证码"`
}

// ProviderSignatureListRequest 签名/标题资源列表查询请求
type ProviderSignatureListRequest struct {
	ProviderAccountID uint  `form:"provider_account_id"`
	Status            *int8 `form:"status" binding:"omitempty,oneof=0 1"`
	Page              int   `form:"page" binding:"omitempty,min=1"`
	PageSize          int   `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// ProviderSignatureListResponse 全局签名/标题资源分页响应
type ProviderSignatureListResponse struct {
	Total int                          `json:"total"`
	Page  int                          `json:"page"`
	Size  int                          `json:"size"`
	Items []*ProviderSignatureResponse `json:"items"`
}

// ProviderSignatureResponse 供应商签名/邮件标题资源响应
type ProviderSignatureResponse struct {
	ID                  uint   `json:"id"`
	ProviderAccountID   uint   `json:"provider_account_id"`
	ProviderAccountName string `json:"provider_account_name,omitempty"`
	ProviderCode        string `json:"provider_code,omitempty"`
	ProviderType        string `json:"provider_type,omitempty"`
	RequiresSignature   bool   `json:"requires_signature"`
	HistoricalOnly      bool   `json:"historical_only"`
	ReadOnly            bool   `json:"read_only"`
	SignatureCode       string `json:"signature_code"`
	SignatureName       string `json:"signature_name"`
	Status              int8   `json:"status"`
	Remark              string `json:"remark,omitempty"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at,omitempty"`
}
