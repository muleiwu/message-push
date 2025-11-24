package dto

import "time"

// CreateProviderSignatureRequest 创建签名请求
type CreateProviderSignatureRequest struct {
	SignatureCode string `json:"signature_code" binding:"required" example:"阿里云"`
	SignatureName string `json:"signature_name" binding:"required" example:"阿里云短信签名"`
	IsDefault     int8   `json:"is_default" example:"0"`
	Status        int8   `json:"status" example:"1"`
	Remark        string `json:"remark" example:"用于发送验证码"`
}

// UpdateProviderSignatureRequest 更新签名请求
type UpdateProviderSignatureRequest struct {
	SignatureCode string `json:"signature_code" binding:"required" example:"阿里云"`
	SignatureName string `json:"signature_name" binding:"required" example:"阿里云短信签名"`
	Status        int8   `json:"status" example:"1"`
	Remark        string `json:"remark" example:"用于发送验证码"`
}

// ProviderSignatureListRequest 签名列表查询请求
type ProviderSignatureListRequest struct {
	ProviderAccountID uint  `form:"provider_account_id"`
	Status            *int8 `form:"status"`
}

// ProviderSignatureResponse 签名响应
type ProviderSignatureResponse struct {
	ID                uint      `json:"id"`
	ProviderAccountID uint      `json:"provider_account_id"`
	SignatureCode     string    `json:"signature_code"`
	SignatureName     string    `json:"signature_name"`
	Status            int8      `json:"status"`
	IsDefault         int8      `json:"is_default"`
	Remark            string    `json:"remark"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// SetDefaultSignatureRequest 设置默认签名请求
type SetDefaultSignatureRequest struct {
	// 可以为空，允许通过路径参数传递
}
