package dto

// CallbackListRequest 服务商回调记录列表请求参数（统一下行回执与上行短信）
type CallbackListRequest struct {
	Page      int    `form:"page" binding:"required,min=1"`
	PageSize  int    `form:"page_size" binding:"required,min=1,max=100"`
	Type      string `form:"type"`       // report=下行回执, upstream=上行短信（空=全部）
	AppID     string `form:"app_id"`     //
	Mobile    string `form:"mobile"`     //
	StartDate string `form:"start_date"` // YYYY-MM-DD
	EndDate   string `form:"end_date"`   // YYYY-MM-DD
}

// CallbackListResponse 服务商回调记录列表响应
type CallbackListResponse struct {
	Total int64           `json:"total"`
	Page  int             `json:"page"`
	Size  int             `json:"size"`
	Items []*CallbackItem `json:"items"`
}

// CallbackItem 服务商回调记录项
type CallbackItem struct {
	ID             uint   `json:"id"`
	Type           string `json:"type"`
	TaskID         string `json:"task_id"`
	AppID          string `json:"app_id"`
	AppName        string `json:"app_name"`
	ProviderCode   string `json:"provider_code"`
	ProviderID     string `json:"provider_id"`
	Mobile         string `json:"mobile"`
	Content        string `json:"content"`
	CallbackStatus string `json:"callback_status"`
	ErrorCode      string `json:"error_code"`
	ErrorMessage   string `json:"error_message"`
	RawData        string `json:"raw_data"`
	CreatedAt      string `json:"created_at"`
}
