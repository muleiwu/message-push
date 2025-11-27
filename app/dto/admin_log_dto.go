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

// TaskLogsResponse 任务日志响应（按task_id查询，不分页）
type TaskLogsResponse struct {
	Items []*LogItem `json:"items"`
}

// CallbackLogItem 回调日志项
type CallbackLogItem struct {
	ID             uint   `json:"id"`
	TaskID         string `json:"task_id"`
	AppID          string `json:"app_id"`
	ProviderCode   string `json:"provider_code"`
	ProviderID     string `json:"provider_id"`
	CallbackStatus string `json:"callback_status"`
	ErrorCode      string `json:"error_code"`
	ErrorMessage   string `json:"error_message"`
	RawData        string `json:"raw_data"`
	CreatedAt      string `json:"created_at"`
}

// TaskCallbackLogsResponse 任务回调日志响应
type TaskCallbackLogsResponse struct {
	Items []*CallbackLogItem `json:"items"`
}

// WebhookLogItem Webhook日志项
type WebhookLogItem struct {
	ID              uint   `json:"id"`
	TaskID          string `json:"task_id"`
	AppID           string `json:"app_id"`
	WebhookConfigID uint   `json:"webhook_config_id"`
	WebhookURL      string `json:"webhook_url"`
	Event           string `json:"event"`
	RequestData     string `json:"request_data"`
	ResponseStatus  int    `json:"response_status"`
	ResponseData    string `json:"response_data"`
	Status          string `json:"status"`
	ErrorMessage    string `json:"error_message"`
	RetryCount      int    `json:"retry_count"`
	CreatedAt       string `json:"created_at"`
}

// TaskWebhookLogsResponse 任务Webhook日志响应
type TaskWebhookLogsResponse struct {
	Items []*WebhookLogItem `json:"items"`
}
