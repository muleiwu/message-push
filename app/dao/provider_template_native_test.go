package dao

import (
	"errors"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
	migrationFS "cnb.cool/mliev/push/message-push/migrations"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func TestLocalNativeContentUpdateAndRollback(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/templates.db"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	goose.SetBaseFS(migrationFS.FS())
	if err := goose.Up(sqlDB, "sqlite"); err != nil {
		t.Fatal(err)
	}
	account := &model.ProviderAccount{AccountCode: "native", ProviderType: "sms"}
	if err := db.Create(account).Error; err != nil {
		t.Fatal(err)
	}
	record := &model.ProviderTemplate{ProviderID: account.ID, TemplateCode: "11", TemplateName: "保留", TemplateContent: "主机{1}", ContentVersion: 1}
	if err := db.Create(record).Error; err != nil {
		t.Fatal(err)
	}
	b := &model.ChannelTemplateBinding{ProviderTemplateID: record.ID, ParamMapping: `[{"type":"fixed","provider_var":"1","value":"host"}]`, MappedContentVersion: 1}
	if err := db.Create(b).Error; err != nil {
		t.Fatal(err)
	}
	d := &ProviderTemplateDAO{db: db}
	record.Remark = "备注变化"
	if err := d.Update(record); err != nil {
		t.Fatal(err)
	}
	db.First(b, b.ID)
	if b.MappedContentVersion != 1 || record.ContentVersion != 1 {
		t.Fatal("metadata update reset mappings")
	}
	if err := db.Callback().Update().Before("gorm:update").Register("fail_template_update", func(tx *gorm.DB) {
		if tx.Statement.Table == "provider_templates" {
			tx.AddError(errors.New("write failed"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	record.TemplateContent += "。"
	if err := d.Update(record); err == nil {
		t.Fatal("expected template failure")
	}
	var saved model.ProviderTemplate
	db.First(&saved, record.ID)
	db.First(b, b.ID)
	if saved.ContentVersion != 1 || saved.TemplateContent != "主机{1}" || b.MappedContentVersion != 1 || b.ParamMapping == "[]" {
		t.Fatal("failed transaction partially reset template/binding")
	}
	db.Callback().Update().Remove("fail_template_update")
	if err := d.Update(record); err != nil {
		t.Fatal(err)
	}
	db.First(&saved, record.ID)
	db.First(b, b.ID)
	if saved.ContentVersion != 2 || b.MappedContentVersion != 0 || b.ParamMapping != "[]" {
		t.Fatal("body change did not atomically reset mapping")
	}
}
