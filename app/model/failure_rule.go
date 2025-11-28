package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// 规则场景常量
const (
	RuleSceneSendFailure     = "send_failure"     // 发送失败场景
	RuleSceneCallbackFailure = "callback_failure" // 回调失败场景
)

// 规则动作常量
const (
	RuleActionRetry          = "retry"           // 重试
	RuleActionSwitchProvider = "switch_provider" // 切换供应商重试
	RuleActionFail           = "fail"            // 直接失败
	RuleActionAlert          = "alert"           // 告警通知
)

// FailureRule 失败处理规则表
type FailureRule struct {
	ID           uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string         `gorm:"type:varchar(100);not null;comment:规则名称" json:"name"`
	Scene        string         `gorm:"type:varchar(20);not null;index:idx_scene_priority;comment:场景：send_failure/callback_failure" json:"scene"`
	ProviderCode string         `gorm:"type:varchar(50);index:idx_provider_code;comment:供应商代码（空=匹配所有）" json:"provider_code"`
	MessageType  string         `gorm:"type:varchar(20);index:idx_message_type;comment:消息类型（空=匹配所有）" json:"message_type"`
	ErrorCode    string         `gorm:"type:varchar(200);comment:错误码（支持逗号分隔多个）" json:"error_code"`
	ErrorKeyword string         `gorm:"type:varchar(200);comment:错误消息关键字（模糊匹配）" json:"error_keyword"`
	Action       string         `gorm:"type:varchar(20);not null;comment:动作：retry/switch_provider/fail/alert" json:"action"`
	ActionConfig string         `gorm:"type:json;comment:动作配置JSON" json:"action_config"`
	Priority     int            `gorm:"type:int;default:0;index:idx_scene_priority;comment:优先级（数字越大越优先）" json:"priority"`
	Status       int8           `gorm:"type:tinyint;default:1;index:idx_status;comment:状态：1=启用 0=禁用" json:"status"`
	Remark       string         `gorm:"type:varchar(500);comment:备注说明" json:"remark"`
	CreatedAt    time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// TableName 指定表名
func (FailureRule) TableName() string {
	return "failure_rules"
}

// RetryActionConfig 重试动作配置
type RetryActionConfig struct {
	MaxRetry     int `json:"max_retry"`     // 最大重试次数
	DelaySeconds int `json:"delay_seconds"` // 重试延迟秒数
	BackoffRate  int `json:"backoff_rate"`  // 退避倍率（指数退避）
	MaxDelay     int `json:"max_delay"`     // 最大延迟秒数
}

// SwitchProviderActionConfig 切换供应商动作配置
type SwitchProviderActionConfig struct {
	ExcludeCurrent bool `json:"exclude_current"` // 是否排除当前供应商
	MaxRetry       int  `json:"max_retry"`       // 切换后最大重试次数
}

// AlertActionConfig 告警动作配置
type AlertActionConfig struct {
	WebhookURL string `json:"webhook_url"` // 告警webhook地址（空则使用系统默认）
	AlertLevel string `json:"alert_level"` // 告警级别：info/warning/critical
}

// GetRetryConfig 获取重试配置
func (r *FailureRule) GetRetryConfig() (*RetryActionConfig, error) {
	if r.ActionConfig == "" {
		return &RetryActionConfig{
			MaxRetry:     3,
			DelaySeconds: 2,
			BackoffRate:  2,
			MaxDelay:     60,
		}, nil
	}
	var config RetryActionConfig
	if err := json.Unmarshal([]byte(r.ActionConfig), &config); err != nil {
		return nil, err
	}
	// 设置默认值
	if config.MaxRetry == 0 {
		config.MaxRetry = 3
	}
	if config.DelaySeconds == 0 {
		config.DelaySeconds = 2
	}
	if config.BackoffRate == 0 {
		config.BackoffRate = 2
	}
	if config.MaxDelay == 0 {
		config.MaxDelay = 60
	}
	return &config, nil
}

// GetSwitchProviderConfig 获取切换供应商配置
func (r *FailureRule) GetSwitchProviderConfig() (*SwitchProviderActionConfig, error) {
	if r.ActionConfig == "" {
		return &SwitchProviderActionConfig{
			ExcludeCurrent: true,
			MaxRetry:       1,
		}, nil
	}
	var config SwitchProviderActionConfig
	if err := json.Unmarshal([]byte(r.ActionConfig), &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetAlertConfig 获取告警配置
func (r *FailureRule) GetAlertConfig() (*AlertActionConfig, error) {
	if r.ActionConfig == "" {
		return &AlertActionConfig{
			AlertLevel: "warning",
		}, nil
	}
	var config AlertActionConfig
	if err := json.Unmarshal([]byte(r.ActionConfig), &config); err != nil {
		return nil, err
	}
	if config.AlertLevel == "" {
		config.AlertLevel = "warning"
	}
	return &config, nil
}

// IsValidScene 检查场景是否有效
func IsValidRuleScene(scene string) bool {
	switch scene {
	case RuleSceneSendFailure, RuleSceneCallbackFailure:
		return true
	default:
		return false
	}
}

// IsValidAction 检查动作是否有效
func IsValidRuleAction(action string) bool {
	switch action {
	case RuleActionRetry, RuleActionSwitchProvider, RuleActionFail, RuleActionAlert:
		return true
	default:
		return false
	}
}
