package dto

import "time"

// PushTaskListRequest 推送任务列表请求参数
type PushTaskListRequest struct {
	Page        int    `form:"page" binding:"required,min=1"`
	PageSize    int    `form:"page_size" binding:"required,min=1,max=100"`
	TaskID      string `form:"task_id"`
	AppID       string `form:"app_id"`
	Status      string `form:"status"`
	MessageType string `form:"message_type"`
	StartDate   string `form:"start_date"` // YYYY-MM-DD
	EndDate     string `form:"end_date"`   // YYYY-MM-DD
	BatchID     string `form:"batch_id"`   // 按批次ID查询
}

// PushTaskListResponse 推送任务列表响应
type PushTaskListResponse struct {
	Total int64           `json:"total"`
	Page  int             `json:"page"`
	Size  int             `json:"size"`
	Items []*PushTaskItem `json:"items"`
}

// PushTaskItem 推送任务项
type PushTaskItem struct {
	ID                uint       `json:"id"`
	TaskID            string     `json:"task_id"`
	AppID             string     `json:"app_id"`
	ChannelID         uint       `json:"channel_id"`
	ProviderAccountID *uint      `json:"provider_account_id"`
	ProviderMsgID     string     `json:"provider_msg_id"`
	MessageType       string     `json:"message_type"`
	Receiver          string     `json:"receiver"`
	Title             string     `json:"title"`
	Content           string     `json:"content"`
	TemplateCode      string     `json:"template_code"`
	TemplateParams    string     `json:"template_params"`
	Signature         string     `json:"signature"`
	Status            string     `json:"status"`
	CallbackStatus    string     `json:"callback_status"`
	CallbackTime      *time.Time `json:"callback_time"`
	RetryCount        int        `json:"retry_count"`
	MaxRetry          int        `json:"max_retry"`
	ScheduledAt       *time.Time `json:"scheduled_at"`
	CreatedAt         string     `json:"created_at"`
	UpdatedAt         string     `json:"updated_at"`
	// 关联数据
	ChannelName         string `json:"channel_name,omitempty"`
	ProviderAccountName string `json:"provider_account_name,omitempty"`
}

// PushBatchTaskListRequest 批量任务列表请求参数
type PushBatchTaskListRequest struct {
	Page      int    `form:"page" binding:"required,min=1"`
	PageSize  int    `form:"page_size" binding:"required,min=1,max=100"`
	BatchID   string `form:"batch_id"`
	AppID     string `form:"app_id"`
	Status    string `form:"status"`
	StartDate string `form:"start_date"` // YYYY-MM-DD
	EndDate   string `form:"end_date"`   // YYYY-MM-DD
}

// PushBatchTaskListResponse 批量任务列表响应
type PushBatchTaskListResponse struct {
	Total int64                `json:"total"`
	Page  int                  `json:"page"`
	Size  int                  `json:"size"`
	Items []*PushBatchTaskItem `json:"items"`
}

// PushBatchTaskItem 批量任务项
type PushBatchTaskItem struct {
	ID           uint   `json:"id"`
	BatchID      string `json:"batch_id"`
	AppID        string `json:"app_id"`
	TotalCount   int    `json:"total_count"`
	SuccessCount int    `json:"success_count"`
	FailedCount  int    `json:"failed_count"`
	PendingCount int    `json:"pending_count"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	// 计算字段
	CompletionRate float64 `json:"completion_rate"`
}
