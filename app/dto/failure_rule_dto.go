package dto

// CreateFailureRuleRequest 创建失败规则请求
type CreateFailureRuleRequest struct {
	Name         string `json:"name" binding:"required,min=2,max=100"`
	Scene        string `json:"scene" binding:"required,oneof=send_failure callback_failure"`
	ProviderCode string `json:"provider_code" binding:"omitempty,max=50"`
	MessageType  string `json:"message_type" binding:"omitempty,oneof=sms email wechat_work dingtalk"`
	ErrorCode    string `json:"error_code" binding:"omitempty,max=200"`
	ErrorKeyword string `json:"error_keyword" binding:"omitempty,max=200"`
	Action       string `json:"action" binding:"required,oneof=retry switch_provider fail alert"`
	ActionConfig string `json:"action_config" binding:"omitempty"`
	Priority     int    `json:"priority" binding:"omitempty,min=0,max=1000"`
	Status       int    `json:"status" binding:"omitempty,oneof=0 1"`
	Remark       string `json:"remark" binding:"omitempty,max=500"`
}

// UpdateFailureRuleRequest 更新失败规则请求
type UpdateFailureRuleRequest struct {
	Name         string `json:"name" binding:"omitempty,min=2,max=100"`
	Scene        string `json:"scene" binding:"omitempty,oneof=send_failure callback_failure"`
	ProviderCode string `json:"provider_code" binding:"omitempty,max=50"`
	MessageType  string `json:"message_type" binding:"omitempty,oneof=sms email wechat_work dingtalk"`
	ErrorCode    string `json:"error_code" binding:"omitempty,max=200"`
	ErrorKeyword string `json:"error_keyword" binding:"omitempty,max=200"`
	Action       string `json:"action" binding:"omitempty,oneof=retry switch_provider fail alert"`
	ActionConfig string `json:"action_config" binding:"omitempty"`
	Priority     int    `json:"priority" binding:"omitempty,min=0,max=1000"`
	Status       *int   `json:"status" binding:"omitempty,oneof=0 1"`
	Remark       string `json:"remark" binding:"omitempty,max=500"`
}

// FailureRuleListRequest 失败规则列表请求
type FailureRuleListRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Scene    string `form:"scene" binding:"omitempty,oneof=send_failure callback_failure"`
	Status   int    `form:"status" binding:"omitempty,oneof=0 1"`
}

// FailureRuleResponse 失败规则响应
type FailureRuleResponse struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Scene        string `json:"scene"`
	SceneLabel   string `json:"scene_label"`
	ProviderCode string `json:"provider_code"`
	MessageType  string `json:"message_type"`
	ErrorCode    string `json:"error_code"`
	ErrorKeyword string `json:"error_keyword"`
	Action       string `json:"action"`
	ActionLabel  string `json:"action_label"`
	ActionConfig string `json:"action_config"`
	Priority     int    `json:"priority"`
	Status       int8   `json:"status"`
	Remark       string `json:"remark"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// FailureRuleListResponse 失败规则列表响应
type FailureRuleListResponse struct {
	Total int                    `json:"total"`
	Page  int                    `json:"page"`
	Size  int                    `json:"size"`
	Items []*FailureRuleResponse `json:"items"`
}

// FailureRuleOptionsResponse 失败规则选项响应
type FailureRuleOptionsResponse struct {
	Scenes  []OptionItem `json:"scenes"`
	Actions []OptionItem `json:"actions"`
}

// OptionItem 选项项
type OptionItem struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
