package dto

// LogListRequest 日志列表请求参数
type LogListRequest struct {
	Page       int    `form:"page" binding:"required,min=1"`
	PageSize   int    `form:"page_size" binding:"required,min=1,max=100"`
	TaskID     string `form:"task_id"`
	AppID      string `form:"app_id"`
	Status     string `form:"status"`
	StartDate  string `form:"start_date"` // YYYY-MM-DD
	EndDate    string `form:"end_date"`   // YYYY-MM-DD
	ProviderID uint   `form:"provider_id"`
}

// LogListResponse 日志列表响应
type LogListResponse struct {
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
	Items []*LogItem `json:"items"`
}

// LogItem 日志项
type LogItem struct {
	ID                uint   `json:"id"`
	TaskID            string `json:"task_id"`
	AppID             string `json:"app_id"`
	AppName           string `json:"app_name"`
	ProviderAccountID uint   `json:"provider_account_id"`
	ProviderName      string `json:"provider_name"`
	RequestData       string `json:"request_data"`
	ResponseData      string `json:"response_data"`
	Status            string `json:"status"`
	ErrorMessage      string `json:"error_message"`
	CostTime          int    `json:"cost_time"`
	CreatedAt         string `json:"created_at"`
}
