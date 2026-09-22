package dao

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCreateWithAttachmentsPersistsSharedGroupAndVerifiesContent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "attachments.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.Exec(`CREATE TABLE push_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL UNIQUE, app_id TEXT NOT NULL,
		channel_id INTEGER NOT NULL, provider_account_id INTEGER, message_type TEXT NOT NULL, receiver TEXT NOT NULL,
		template_code TEXT, template_params TEXT, signature TEXT, attachment_group_id TEXT, status TEXT,
		callback_status TEXT, callback_time DATETIME, retry_count INTEGER, max_retry INTEGER,
		exclude_provider_ids TEXT, scheduled_at DATETIME, created_at DATETIME, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create push_tasks test schema: %v", err)
	}
	if err := db.AutoMigrate(&model.EmailAttachment{}); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}

	content := []byte("attachment-content")
	digest := sha256.Sum256(content)
	groupID := "group-1"
	attachment := &model.EmailAttachment{
		AttachmentGroupID: groupID,
		Position:          0,
		Filename:          "file.txt",
		ContentType:       "text/plain",
		SizeBytes:         int64(len(content)),
		SHA256:            hex.EncodeToString(digest[:]),
		Content:           content,
	}
	first := &model.PushTask{
		TaskID:            "task-with-attachment",
		AppID:             "app",
		ChannelID:         1,
		MessageType:       constants.MessageTypeEmail,
		Receiver:          "first@example.com",
		AttachmentGroupID: groupID,
		Status:            constants.TaskStatusPending,
	}
	dao := NewPushTaskDAOWithDB(db)
	if err := dao.CreateWithAttachments(first, []*model.EmailAttachment{attachment}); err != nil {
		t.Fatalf("CreateWithAttachments() error = %v", err)
	}
	second := &model.PushTask{
		TaskID:            "task-sharing-attachment",
		AppID:             "app",
		ChannelID:         1,
		MessageType:       constants.MessageTypeEmail,
		Receiver:          "second@example.com",
		AttachmentGroupID: groupID,
		Status:            constants.TaskStatusPending,
	}
	if err := dao.Create(second); err != nil {
		t.Fatalf("create shared task: %v", err)
	}

	for _, taskID := range []string{first.TaskID, second.TaskID} {
		persisted, err := dao.GetByTaskID(taskID)
		if err != nil || persisted.AttachmentGroupID != groupID {
			t.Fatalf("task %s attachment group = %q, error=%v", taskID, persisted.AttachmentGroupID, err)
		}
	}
	loaded, err := NewEmailAttachmentDAOWithDB(db).GetForSend(groupID)
	if err != nil || len(loaded) != 1 || string(loaded[0].Content) != string(content) {
		t.Fatalf("GetForSend() = %+v, %v", loaded, err)
	}
	var attachmentCount int64
	if err := db.Model(&model.EmailAttachment{}).Count(&attachmentCount).Error; err != nil || attachmentCount != 1 {
		t.Fatalf("attachment count = %d, error=%v", attachmentCount, err)
	}
}

func TestGetForSendFailsClosedForMissingOrCorruptAttachment(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "corrupt.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.AutoMigrate(&model.EmailAttachment{}); err != nil {
		t.Fatalf("migrate test schema: %v", err)
	}
	dao := NewEmailAttachmentDAOWithDB(db)
	if _, err := dao.GetForSend("missing"); err == nil {
		t.Fatal("missing attachment group did not fail closed")
	}
	if err := db.Create(&model.EmailAttachment{
		AttachmentGroupID: "corrupt",
		Position:          0,
		Filename:          "file.txt",
		ContentType:       "text/plain",
		SizeBytes:         4,
		SHA256:            "wrong",
		Content:           []byte("data"),
	}).Error; err != nil {
		t.Fatalf("create corrupt attachment: %v", err)
	}
	if _, err := dao.GetForSend("corrupt"); err == nil {
		t.Fatal("corrupt attachment did not fail closed")
	}
}
