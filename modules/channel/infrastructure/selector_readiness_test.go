package infrastructure

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/readiness"
	channelDomain "cnb.cool/mliev/push/message-push/modules/channel/domain"
	registry "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"github.com/glebarez/sqlite"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
)

const testSelectorReadinessProvider = "selector_readiness_plain"

func TestFilterEligibleNodesRevalidatesCachedNodeAfterHardStop(t *testing.T) {
	if err := registry.Register(&registry.ProviderMeta{
		Code: testSelectorReadinessProvider,
		Name: "selector readiness",
		Type: constants.MessageTypeSMS,
	}); err != nil && !strings.Contains(err.Error(), "already registered") {
		t.Fatal(err)
	}
	db := newSelectorReadinessDB(t)

	template := &model.MessageTemplate{TemplateName: "system", Content: "code={code}", Status: 1}
	if err := template.SetVariables([]string{"code"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(template).Error; err != nil {
		t.Fatal(err)
	}
	channel := &model.Channel{Name: "sms", Type: constants.MessageTypeSMS, MessageTemplateID: template.ID, Status: 1}
	if err := db.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	account := &model.ProviderAccount{
		AccountCode:  "selector-account",
		AccountName:  "selector account",
		ProviderCode: testSelectorReadinessProvider,
		ProviderType: constants.MessageTypeSMS,
		Config:       `{}`,
		Status:       1,
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatal(err)
	}
	providerTemplate := &model.ProviderTemplate{ProviderID: account.ID, TemplateCode: "provider-template", TemplateName: "provider", Status: 1}
	if err := providerTemplate.SetVariables([]string{"code"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(providerTemplate).Error; err != nil {
		t.Fatal(err)
	}
	binding := &model.ChannelTemplateBinding{
		ChannelID:          channel.ID,
		ProviderTemplateID: providerTemplate.ID,
		ProviderID:         account.ID,
		Weight:             10,
		Priority:           100,
		Status:             1,
		IsActive:           1,
	}
	if err := db.Create(binding).Error; err != nil {
		t.Fatal(err)
	}

	selector := &ChannelSelector{readinessEvaluator: readiness.NewChannelEvaluator(db)}
	cachedNodes := []*channelDomain.ChannelNode{{
		ChannelTemplateBinding: binding,
		ProviderAccount:        account,
	}}
	filtered, err := selector.filterEligibleNodes(channel.ID, cachedNodes)
	if err != nil || len(filtered) != 1 {
		t.Fatalf("ready cached node was filtered: nodes=%d err=%v", len(filtered), err)
	}

	if err := db.Model(&model.Channel{}).Where("id = ?", channel.ID).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	filtered, err = selector.filterEligibleNodes(channel.ID, cachedNodes)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 0 {
		t.Fatalf("disabled channel leaked %d cached selector candidates", len(filtered))
	}

	if err := db.Model(&model.Channel{}).Where("id = ?", channel.ID).Update("status", 1).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.MessageTemplate{}).Where("id = ?", template.ID).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	filtered, err = selector.filterEligibleNodes(channel.ID, cachedNodes)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 0 {
		t.Fatalf("disabled template leaked %d cached selector candidates", len(filtered))
	}
}

func TestClearCacheByChannelIDCoversEverySupportedMessageType(t *testing.T) {
	cache := &recordingSelectorCache{deleted: make(map[string]struct{})}
	selector := &ChannelSelector{cache: cache, logger: noopSelectorLogger{}}
	const channelID uint = 42

	selector.ClearCacheByChannelID(channelID)

	wantTypes := append(constants.SupportedMessageTypes(), "")
	for _, messageType := range wantTypes {
		key := buildCacheKey(channelID, messageType)
		if _, ok := cache.deleted[key]; !ok {
			t.Errorf("cache key %q was not invalidated", key)
		}
	}
	if len(cache.deleted) != len(wantTypes) {
		t.Fatalf("deleted %d cache keys, want %d: %v", len(cache.deleted), len(wantTypes), cache.deleted)
	}
}

func newSelectorReadinessDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "selector.db")), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE message_templates (id INTEGER PRIMARY KEY AUTOINCREMENT, template_name TEXT NOT NULL, content_type TEXT, content TEXT, variables TEXT, description TEXT, status INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE channels (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, type TEXT NOT NULL, message_template_id INTEGER, status INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_accounts (id INTEGER PRIMARY KEY AUTOINCREMENT, account_code TEXT NOT NULL UNIQUE, account_name TEXT NOT NULL, provider_code TEXT NOT NULL, provider_type TEXT NOT NULL, config TEXT, status INTEGER, remark TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_templates (id INTEGER PRIMARY KEY AUTOINCREMENT, provider_id INTEGER NOT NULL, template_code TEXT NOT NULL, template_name TEXT NOT NULL, content_type TEXT, template_content TEXT, variables TEXT, status INTEGER, remark TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE channel_template_bindings (id INTEGER PRIMARY KEY AUTOINCREMENT, channel_id INTEGER NOT NULL, provider_template_id INTEGER NOT NULL, provider_id INTEGER NOT NULL, param_mapping TEXT, weight INTEGER, priority INTEGER, status INTEGER, is_active INTEGER, auto_disable_on_fail INTEGER, auto_disable_threshold INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_signatures (id INTEGER PRIMARY KEY AUTOINCREMENT, provider_account_id INTEGER NOT NULL, signature_code TEXT NOT NULL, signature_name TEXT NOT NULL, status INTEGER, remark TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE channel_signature_mappings (id INTEGER PRIMARY KEY AUTOINCREMENT, channel_id INTEGER NOT NULL, signature_name TEXT NOT NULL, provider_signature_id INTEGER NOT NULL, provider_id INTEGER NOT NULL, status INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create selector schema: %v", err)
		}
	}
	return db
}

type recordingSelectorCache struct {
	deleted map[string]struct{}
}

func (c *recordingSelectorCache) Exists(context.Context, string) bool { return false }
func (c *recordingSelectorCache) Get(context.Context, string, any) error {
	return nil
}
func (c *recordingSelectorCache) Set(context.Context, string, any, time.Duration) error {
	return nil
}
func (c *recordingSelectorCache) GetSet(context.Context, string, time.Duration, any, gsr.CacheCallback) error {
	return nil
}
func (c *recordingSelectorCache) Del(_ context.Context, key string) error {
	c.deleted[key] = struct{}{}
	return nil
}
func (c *recordingSelectorCache) ExpiresAt(context.Context, string, time.Time) error {
	return nil
}
func (c *recordingSelectorCache) ExpiresIn(context.Context, string, time.Duration) error {
	return nil
}

type noopSelectorLogger struct{}

func (noopSelectorLogger) Debug(string, ...gsr.LoggerField)  {}
func (noopSelectorLogger) Info(string, ...gsr.LoggerField)   {}
func (noopSelectorLogger) Notice(string, ...gsr.LoggerField) {}
func (noopSelectorLogger) Error(string, ...gsr.LoggerField)  {}
func (noopSelectorLogger) Warn(string, ...gsr.LoggerField)   {}
func (noopSelectorLogger) Fatal(string, ...gsr.LoggerField)  {}
