package dto

// CreateProviderSignatureRequest 创建签名请求
type CreateProviderSignatureRequest struct {
	// SignatureCode 签名代码：实际发送用，原样提交给供应商作为短信签名，须与供应商平台报备审核通过的签名一致
	SignatureCode string `json:"signature_code" binding:"required" example:"墨蕾科技"`
	// SignatureName 签名名称：仅后台展示用，不参与实际发送，仅用于后台识别。通常与签名代码相同
	SignatureName string `json:"signature_name" binding:"required" example:"墨蕾科技验证码签名"`
	Status        int8   `json:"status" example:"1"`
	Remark        string `json:"remark" example:"用于发送验证码"`
}

// UpdateProviderSignatureRequest 更新签名请求
type UpdateProviderSignatureRequest struct {
	// SignatureCode 签名代码：实际发送用，原样提交给供应商作为短信签名，须与供应商平台报备审核通过的签名一致
	SignatureCode string `json:"signature_code" binding:"required" example:"墨蕾科技"`
	// SignatureName 签名名称：仅后台展示用，不参与实际发送，仅用于后台识别。通常与签名代码相同
	SignatureName string `json:"signature_name" binding:"required" example:"墨蕾科技验证码签名"`
	Status        int8   `json:"status" example:"1"`
	Remark        string `json:"remark" example:"用于发送验证码"`
}

// ProviderSignatureListRequest 签名列表查询请求
type ProviderSignatureListRequest struct {
	ProviderAccountID uint  `form:"provider_account_id"`
	Status            *int8 `form:"status" binding:"omitempty,oneof=0 1"`
	Page              int   `form:"page" binding:"omitempty,min=1"`
	PageSize          int   `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// ProviderSignatureListResponse 全局签名分页响应
type ProviderSignatureListResponse struct {
	Total int                          `json:"total"`
	Page  int                          `json:"page"`
	Size  int                          `json:"size"`
	Items []*ProviderSignatureResponse `json:"items"`
}

// ProviderSignatureResponse 签名响应
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
