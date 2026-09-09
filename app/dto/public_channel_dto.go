package dto

// PublicChannelListRequest filters the channel catalog available to API applications.
type PublicChannelListRequest struct {
	Type     string `form:"type"`
	Page     int    `form:"page" binding:"min=1"`
	PageSize int    `form:"page_size" binding:"min=1,max=100"`
}

// PublicChannelReadiness exposes delivery state without internal resource IDs.
type PublicChannelReadiness struct {
	State        string   `json:"state"`
	BlockerCodes []string `json:"blocker_codes"`
}

// PublicChannelResponse identifies a channel and its bound system template.
type PublicChannelResponse struct {
	ID                uint                   `json:"id"`
	Name              string                 `json:"name"`
	Type              string                 `json:"type"`
	MessageTemplateID uint                   `json:"message_template_id"`
	TemplateName      string                 `json:"template_name"`
	Readiness         PublicChannelReadiness `json:"readiness"`
}

// PublicChannelListResponse is a page of enabled channels, including blocked ones.
type PublicChannelListResponse struct {
	Items []PublicChannelResponse `json:"items"`
	Total int64                   `json:"total"`
	Page  int                     `json:"page"`
	Size  int                     `json:"size"`
}

// PublicChannelTemplate contains the system template used to build template_params.
// Variables is null for invalid configuration and [] for a valid template without variables.
type PublicChannelTemplate struct {
	ID           uint     `json:"id"`
	TemplateName string   `json:"template_name"`
	ContentType  string   `json:"content_type"`
	Content      string   `json:"content"`
	Variables    []string `json:"variables"`
	Description  string   `json:"description"`
}

// PublicChannelDetailResponse provides all options needed by a message form.
type PublicChannelDetailResponse struct {
	PublicChannelResponse
	Template          *PublicChannelTemplate `json:"template"`
	SignatureRequired bool                   `json:"signature_required"`
	SignatureNames    []string               `json:"signature_names"`
}
