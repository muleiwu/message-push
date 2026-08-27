package dao

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	webHelper "cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"gorm.io/gorm"
)

// EmailAttachmentDAO 管理异步邮件附件及其终态清理。
type EmailAttachmentDAO struct {
	db *gorm.DB
}

func NewEmailAttachmentDAO() *EmailAttachmentDAO {
	return NewEmailAttachmentDAOWithDB(webHelper.GetDatabase())
}

func NewEmailAttachmentDAOWithDB(db *gorm.DB) *EmailAttachmentDAO {
	return &EmailAttachmentDAO{db: db}
}

func (d *EmailAttachmentDAO) CreateMany(attachments []*model.EmailAttachment) error {
	if len(attachments) == 0 {
		return nil
	}
	return d.db.Create(&attachments).Error
}

// GetForSend 加载并校验附件内容，缺失、已清理或损坏时 fail-closed。
func (d *EmailAttachmentDAO) GetForSend(groupID string) ([]*model.EmailAttachment, error) {
	if groupID == "" {
		return nil, nil
	}
	var attachments []*model.EmailAttachment
	if err := d.db.Where("attachment_group_id = ?", groupID).Order("position ASC").Find(&attachments).Error; err != nil {
		return nil, err
	}
	if len(attachments) == 0 {
		return nil, fmt.Errorf("email attachment group %s was not found", groupID)
	}
	for _, attachment := range attachments {
		if attachment.PurgedAt != nil || attachment.Content == nil {
			return nil, fmt.Errorf("email attachment %q is no longer available", attachment.Filename)
		}
		if int64(len(attachment.Content)) != attachment.SizeBytes {
			return nil, fmt.Errorf("email attachment %q size verification failed", attachment.Filename)
		}
		digest := sha256.Sum256(attachment.Content)
		if hex.EncodeToString(digest[:]) != attachment.SHA256 {
			return nil, fmt.Errorf("email attachment %q integrity verification failed", attachment.Filename)
		}
	}
	return attachments, nil
}

// PurgeIfUnreferenced 清空已全部进入终态的附件组二进制，保留元数据。
func (d *EmailAttachmentDAO) PurgeIfUnreferenced(groupID string, purgedAt time.Time) error {
	if groupID == "" {
		return nil
	}
	var activeTasks int64
	if err := d.db.Model(&model.PushTask{}).
		Where("attachment_group_id = ?", groupID).
		Where("status NOT IN ?", []string{constants.TaskStatusSuccess, constants.TaskStatusFailed}).
		Count(&activeTasks).Error; err != nil {
		return err
	}
	if activeTasks > 0 {
		return nil
	}
	return d.db.Model(&model.EmailAttachment{}).
		Where("attachment_group_id = ? AND purged_at IS NULL", groupID).
		Updates(map[string]any{"content": nil, "purged_at": purgedAt}).Error
}
