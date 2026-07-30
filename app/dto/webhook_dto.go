package dto

// WebhookPayload is the stable outbound payload delivered to applications.
type WebhookPayload struct {
	Event     string                 `json:"event"`
	TaskID    string                 `json:"task_id"`
	AppID     string                 `json:"app_id"`
	Status    string                 `json:"status"`
	Receiver  string                 `json:"receiver"`
	ErrorCode string                 `json:"error_code"`
	ErrorMsg  string                 `json:"error_msg"`
	Timestamp int64                  `json:"timestamp"`
	Extra     map[string]interface{} `json:"extra"`
}
