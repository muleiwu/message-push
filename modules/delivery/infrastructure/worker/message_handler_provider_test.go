package worker

import (
	"path/filepath"
	"testing"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestPersistSelectedProviderOverwritesLastAttempt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "worker-provider.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		t.Cleanup(func() { _ = sqlDB.Close() })
	}
	if err := db.Exec(`CREATE TABLE push_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL UNIQUE,
		provider_account_id INTEGER, updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if err := db.Exec("INSERT INTO push_tasks (task_id) VALUES (?)", "provider-attempt").Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	task := &model.PushTask{TaskID: "provider-attempt"}
	handler := &MessageHandler{taskDao: dao.NewPushTaskDAOWithDB(db)}
	for _, providerID := range []uint{7, 9} {
		if err := handler.persistSelectedProvider(task, providerID); err != nil {
			t.Fatalf("persist provider %d: %v", providerID, err)
		}
		if task.ProviderAccountID == nil || *task.ProviderAccountID != providerID {
			t.Fatalf("in-memory provider = %v, want %d", task.ProviderAccountID, providerID)
		}
	}

	var persisted uint
	if err := db.Table("push_tasks").
		Select("provider_account_id").
		Where("task_id = ?", task.TaskID).
		Scan(&persisted).Error; err != nil {
		t.Fatalf("query provider: %v", err)
	}
	if persisted != 9 {
		t.Fatalf("persisted provider = %d, want 9", persisted)
	}
}
