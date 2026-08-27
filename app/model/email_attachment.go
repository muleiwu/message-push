package model

import "time"

// EmailAttachment 保存异步邮件发送所需的附件内容。
// 同一批次的任务通过 AttachmentGroupID 共享一份附件数据。
type EmailAttachment struct {
	ID                uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	AttachmentGroupID string     `gorm:"type:varchar(36);not null;uniqueIndex:uk_email_attachments_group_position,priority:1;index:idx_email_attachments_group;comment:附件组UUID" json:"attachment_group_id"`
	Position          int        `gorm:"type:int;not null;uniqueIndex:uk_email_attachments_group_position,priority:2;comment:附件顺序" json:"position"`
	Filename          string     `gorm:"type:varchar(255);not null;comment:文件名" json:"filename"`
	ContentType       string     `gorm:"type:varchar(255);not null;comment:MIME类型" json:"content_type"`
	SizeBytes         int64      `gorm:"not null;comment:解码后字节数" json:"size_bytes"`
	SHA256            string     `gorm:"type:char(64);not null;comment:内容SHA-256" json:"sha256"`
	Content           []byte     `gorm:"comment:附件二进制内容" json:"-"`
	PurgedAt          *time.Time `gorm:"comment:二进制清理时间" json:"purged_at"`
	CreatedAt         time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (EmailAttachment) TableName() string {
	return "email_attachments"
}
