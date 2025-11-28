package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// PushTask 推送任务表
type PushTask struct {
	ID                 uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID             string     `gorm:"type:varchar(36);uniqueIndex:uk_task_id;not null;comment:任务UUID" json:"task_id"`
	AppID              string     `gorm:"type:varchar(32);not null;index:idx_app_id_status;comment:应用ID" json:"app_id"`
	ChannelID          uint       `gorm:"type:bigint unsigned;not null;index:idx_channel;comment:通道ID" json:"channel_id"`
	MessageType        string     `gorm:"type:varchar(20);not null;comment:消息类型：sms, email等" json:"message_type"`
	Receiver           string     `gorm:"type:varchar(100);not null;comment:接收者（手机号/邮箱/UserID等）" json:"receiver"`
	Title              string     `gorm:"type:varchar(200);comment:标题（邮件、企微、钉钉使用）" json:"title"`
	Content            string     `gorm:"type:text;comment:内容（直接发送或模板渲染后内容）" json:"content"`
	TemplateCode       string     `gorm:"type:varchar(50);comment:模板代码" json:"template_code"`
	TemplateParams     string     `gorm:"type:json;comment:模板参数" json:"template_params"`
	Signature          string     `gorm:"type:varchar(50);comment:签名" json:"signature"`
	Status             string     `gorm:"type:varchar(20);default:'pending';index:idx_app_id_status,idx_status_scheduled;comment:状态：pending, processing, success, failed" json:"status"`
	CallbackStatus     string     `gorm:"type:varchar(20);comment:回调状态：pending, delivered, failed, rejected" json:"callback_status"`
	CallbackTime       *time.Time `gorm:"type:timestamp;comment:回调时间" json:"callback_time"`
	RetryCount         int        `gorm:"type:int;default:0;comment:已重试次数" json:"retry_count"`
	MaxRetry           int        `gorm:"type:int;default:3;comment:最大重试次数" json:"max_retry"`
	ExcludeProviderIDs string     `gorm:"type:json;comment:排除的供应商账号ID列表（规则引擎切换供应商使用）" json:"exclude_provider_ids"`
	ScheduledAt        *time.Time `gorm:"type:timestamp;index:idx_status_scheduled;comment:定时发送时间" json:"scheduled_at"`
	CreatedAt          time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP;index:idx_created_at" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" json:"updated_at"`
	Channel            *Channel   `gorm:"foreignKey:ChannelID;references:ID" json:"channel,omitempty"`
}

// GetExcludeProviderIDs 获取排除的供应商ID列表
func (t *PushTask) GetExcludeProviderIDs() []uint {
	if t.ExcludeProviderIDs == "" || t.ExcludeProviderIDs == "[]" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(t.ExcludeProviderIDs), &ids); err != nil {
		return nil
	}
	return ids
}

// SetExcludeProviderIDs 设置排除的供应商ID列表
func (t *PushTask) SetExcludeProviderIDs(ids []uint) {
	if len(ids) == 0 {
		t.ExcludeProviderIDs = "[]"
		return
	}
	data, _ := json.Marshal(ids)
	t.ExcludeProviderIDs = string(data)
}

// BeforeSave GORM hook - 确保 ExcludeProviderIDs 是有效的 JSON
func (t *PushTask) BeforeSave(tx *gorm.DB) error {
	// 如果 ExcludeProviderIDs 为空字符串，设置为有效的空 JSON 数组
	if t.ExcludeProviderIDs == "" {
		t.ExcludeProviderIDs = "[]"
	}
	return nil
}

// AddExcludeProviderID 添加一个需要排除的供应商ID
func (t *PushTask) AddExcludeProviderID(id uint) {
	ids := t.GetExcludeProviderIDs()
	// 检查是否已存在
	for _, existingID := range ids {
		if existingID == id {
			return
		}
	}
	ids = append(ids, id)
	t.SetExcludeProviderIDs(ids)
}

// TableName 指定表名
func (PushTask) TableName() string {
	return "push_tasks"
}
