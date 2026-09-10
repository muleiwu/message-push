package admin

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"cnb.cool/mliev/open/go-web/pkg/container"
	httpInterfaces "cnb.cool/mliev/open/go-web/pkg/server/http_server/interfaces"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/modules/channel"
	_ "cnb.cool/mliev/push/message-push/modules/sender/infrastructure"
	"github.com/glebarez/sqlite"
	"github.com/muleiwu/gsr"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const bindingUpdatePayload = `{"param_mapping":[{"type":"mapping","provider_var":"host_name","system_var":"host_name","value":""},{"type":"mapping","provider_var":"dimension","system_var":"dimension","value":""},{"type":"mapping","provider_var":"value","system_var":"value","value":""},{"type":"mapping","provider_var":"threshold","system_var":"threshold","value":""}],"status":1,"weight":10,"priority":100,"is_active":1,"auto_disable_on_fail":false,"auto_disable_threshold":5}`
const pendingTemplateMessage = "供应商模板尚未审核通过，请审核通过并同步后重试"

type bindingControllerContext struct {
	httpInterfaces.RouterContextInterface
	payload string
	status  int
	body    dto.Response
}

func (c *bindingControllerContext) Param(name string) string {
	if name == "bindingId" {
		return "29"
	}
	return "17"
}

func (c *bindingControllerContext) ShouldBindJSON(value any) error {
	return json.Unmarshal([]byte(c.payload), value)
}

func (c *bindingControllerContext) JSON(status int, body any) {
	c.status = status
	c.body = body.(dto.Response)
}

type bindingControllerSelector struct {
	channel.Selector
	invalidated []uint
	reset       []uint
}

func (s *bindingControllerSelector) InvalidateCacheForBinding(id uint) {
	s.invalidated = append(s.invalidated, id)
}

func (s *bindingControllerSelector) ResetWeightsByChannelID(id uint) {
	s.reset = append(s.reset, id)
}

type bindingControllerLogger struct{ gsr.Logger }

func (bindingControllerLogger) Error(string, ...gsr.LoggerField) {}
func (bindingControllerLogger) Info(string, ...gsr.LoggerField)  {}

type bindingControllerProvider[T any] struct {
	value    T
	priority int
}

func (p bindingControllerProvider[T]) Type() reflect.Type { return reflect.TypeFor[T]() }
func (p bindingControllerProvider[T]) Build() any         { return p.value }
func (p bindingControllerProvider[T]) Priority() int      { return p.priority }

// These integration tests run sequentially because the application resolves
// its database and selector through the process-wide DI container.
var bindingControllerProviderPriority = 10000

func registerBindingControllerDependency[T any](value T) {
	bindingControllerProviderPriority++
	container.Register(bindingControllerProvider[T]{value: value, priority: bindingControllerProviderPriority})
}

type bindingControllerFixture struct {
	db       *gorm.DB
	selector *bindingControllerSelector
	existing model.ChannelTemplateBinding
}

func newBindingControllerFixture(t *testing.T, update bool) *bindingControllerFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "bindings.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	for _, statement := range []string{
		`CREATE TABLE message_templates (id INTEGER PRIMARY KEY, template_name TEXT, content_type TEXT, content TEXT, variables TEXT, description TEXT, status INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE channels (id INTEGER PRIMARY KEY, name TEXT, type TEXT, message_template_id INTEGER, status INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_accounts (id INTEGER PRIMARY KEY, account_code TEXT, account_name TEXT, provider_code TEXT, provider_type TEXT, config TEXT, status INTEGER, remark TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_templates (id INTEGER PRIMARY KEY, provider_id INTEGER, template_code TEXT, template_name TEXT, content_type TEXT, template_content TEXT, variables TEXT, status INTEGER, remark TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME, remote_id TEXT, audit_status INTEGER, audit_reply TEXT, remote_deleted INTEGER, synced_at DATETIME, remote_description TEXT, native_content TEXT, variable_slots TEXT, codec_version TEXT, remote_name TEXT, category TEXT)`,
		`CREATE TABLE channel_template_bindings (id INTEGER PRIMARY KEY, channel_id INTEGER, provider_template_id INTEGER, provider_id INTEGER, param_mapping TEXT, weight INTEGER, priority INTEGER, status INTEGER, is_active INTEGER, auto_disable_on_fail INTEGER, auto_disable_threshold INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	const variables = `["host_name","dimension","value","threshold"]`
	const content = "主机{host_name}的磁盘分区{dimension}发生空间告警，当前使用率{value}%，告警阈值{threshold}%，请及时处理。"
	for _, record := range []any{
		&model.MessageTemplate{ID: 8, TemplateName: "磁盘空间告警", Content: content, Variables: variables, Status: 1},
		&model.Channel{ID: 17, Name: "磁盘空间告警", Type: "sms", MessageTemplateID: 8, Status: 1},
		&model.ProviderAccount{ID: 1, AccountCode: "binding-test", ProviderCode: "zrwinfo_sms", ProviderType: "sms", Status: 1, Config: "{}"},
		&model.ProviderTemplate{
			ID: 25, ProviderID: 1, TemplateCode: "3344803", TemplateName: "磁盘空间告警", Status: 1,
			TemplateContent: content, Variables: variables, CodecVersion: "positional-v1",
			NativeContent: "主机{1}的磁盘分区{2}发生空间告警，当前使用率{3}%，告警阈值{4}%，请及时处理。",
			VariableSlots: `[{"native":"1","name":"host_name"},{"native":"2","name":"dimension"},{"native":"3","name":"value"},{"native":"4","name":"threshold"}]`,
		},
	} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	f := &bindingControllerFixture{db: db, selector: &bindingControllerSelector{}}
	if update {
		f.existing = model.ChannelTemplateBinding{ID: 29, ChannelID: 17, ProviderID: 1, ProviderTemplateID: 25, ParamMapping: "[]", Status: 1, IsActive: 1, Weight: 3, Priority: 50}
		if err := db.Create(&f.existing).Error; err != nil {
			t.Fatal(err)
		}
	}
	registerBindingControllerDependency(db)
	registerBindingControllerDependency[channel.Selector](f.selector)
	registerBindingControllerDependency[gsr.Logger](bindingControllerLogger{})
	return f
}

func (f *bindingControllerFixture) request(t *testing.T, update bool, fields map[string]any) *bindingControllerContext {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(bindingUpdatePayload), &payload); err != nil {
		t.Fatal(err)
	}
	if !update {
		payload["provider_id"] = 1
		payload["provider_template_id"] = 25
	}
	for key, value := range fields {
		payload[key] = value
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &bindingControllerContext{payload: string(data)}
	if update {
		ChannelController{}.UpdateChannelBinding(ctx)
	} else {
		ChannelController{}.CreateChannelBinding(ctx)
	}
	return ctx
}

func TestChannelBindingValidationResponses(t *testing.T) {
	states := []struct {
		name    string
		changes map[string]any
		message string
	}{
		{"unknown", map[string]any{"audit_status": 0}, "供应商模板审核状态待确认，请同步审核结果后重试"},
		{"pending", map[string]any{"audit_status": 1}, pendingTemplateMessage},
		{"approved", map[string]any{"audit_status": 2}, ""},
		{"rejected", map[string]any{"audit_status": 3}, "供应商模板审核未通过，请修改并重新提交审核"},
		{"legacy", map[string]any{"audit_status": nil}, ""},
		{"disabled", map[string]any{"status": 0}, "供应商模板已禁用，请启用后重试"},
		{"remote deleted", map[string]any{"remote_deleted": true}, "供应商模板已在远端删除，请更换模板"},
		{"locally deleted", map[string]any{"deleted_at": "2026-09-10 01:00:00"}, "供应商模板不存在或已删除，请更换模板"},
		{"missing code", map[string]any{"template_code": " "}, "供应商模板编号为空，请完善模板配置"},
		{"invalid variables", map[string]any{"variables": `["host_name","host_name"]`}, "供应商模板变量定义异常，请修正配置或重新同步模板"},
		{"invalid slots", map[string]any{"variable_slots": `[]`}, "供应商模板变量定义异常，请修正配置或重新同步模板"},
		{"multiple reasons", map[string]any{"status": 0, "audit_status": 1, "template_code": ""}, "供应商模板已禁用，请启用后重试；" + pendingTemplateMessage + "；供应商模板编号为空，请完善模板配置"},
	}
	for _, update := range []bool{false, true} {
		operation := "create"
		if update {
			operation = "update"
		}
		t.Run(operation, func(t *testing.T) {
			for _, state := range states {
				t.Run(state.name, func(t *testing.T) {
					f := newBindingControllerFixture(t, update)
					if err := f.db.Model(&model.ProviderTemplate{}).Where("id = ?", 25).Updates(state.changes).Error; err != nil {
						t.Fatal(err)
					}
					ctx := f.request(t, update, nil)
					if state.message == "" {
						if ctx.status != 200 || ctx.body.Code != 0 {
							t.Fatalf("response = %d %+v, want success", ctx.status, ctx.body)
						}
						var saved model.ChannelTemplateBinding
						if err := f.db.Where("channel_id = ?", 17).First(&saved).Error; err != nil {
							t.Fatal(err)
						}
						if saved.Weight != 10 || saved.Priority != 100 || saved.Status != 1 || saved.IsActive != 1 {
							t.Fatalf("request not persisted: %+v", saved)
						}
						if len(f.selector.invalidated) != 1 || len(f.selector.reset) != 1 {
							t.Fatal("successful save did not invalidate cache and weights")
						}
					} else {
						f.assertUnchanged(t, update)
						if ctx.status != 400 || ctx.body.Code != 400 || ctx.body.Message != state.message {
							t.Fatalf("response = %d %+v, want 400 %q", ctx.status, ctx.body, state.message)
						}
					}
				})
			}
		})
	}
}

func TestChannelBindingDisabledEditingKeepsExistingRules(t *testing.T) {
	for _, update := range []bool{false, true} {
		for _, field := range []string{"status", "is_active"} {
			t.Run(fmt.Sprintf("update=%t/%s", update, field), func(t *testing.T) {
				f := newBindingControllerFixture(t, update)
				if err := f.db.Model(&model.ProviderTemplate{}).Where("id = ?", 25).Updates(map[string]any{"audit_status": 1, "status": 0, "template_code": ""}).Error; err != nil {
					t.Fatal(err)
				}
				ctx := f.request(t, update, map[string]any{field: 0})
				if ctx.status != 200 {
					t.Fatalf("disabled binding should allow mapping edits: %+v", ctx.body)
				}
				// Invalid mapping data is still rejected, but unchecked template
				// audit/local status and template code must not appear in the error.
				if !update {
					if err := f.db.Where("channel_id = ?", 17).Delete(&model.ChannelTemplateBinding{}).Error; err != nil {
						t.Fatal(err)
					}
				}
				if err := f.db.Model(&model.ProviderTemplate{}).Where("id = ?", 25).Update("variables", `["host_name","host_name"]`).Error; err != nil {
					t.Fatal(err)
				}
				ctx = f.request(t, update, map[string]any{field: 0})
				if ctx.status != 400 || ctx.body.Message != "供应商模板变量定义异常，请修正配置或重新同步模板" {
					t.Fatalf("disabled mapping validation: %+v", ctx.body)
				}
			})
		}
	}
	t.Run("disabled update without mapping skips template validation", func(t *testing.T) {
		f := newBindingControllerFixture(t, true)
		if err := f.db.Delete(&model.ProviderTemplate{}, 25).Error; err != nil {
			t.Fatal(err)
		}
		ctx := f.request(t, true, map[string]any{"status": 0, "param_mapping": nil})
		if ctx.status != 200 {
			t.Fatalf("disabling a binding with no mapping edit must still work: %+v", ctx.body)
		}
	})
}

func (f *bindingControllerFixture) assertUnchanged(t *testing.T, update bool) {
	t.Helper()
	if len(f.selector.invalidated) != 0 || len(f.selector.reset) != 0 {
		t.Fatal("failed validation invalidated cache or weights")
	}
	if update {
		var actual model.ChannelTemplateBinding
		if err := f.db.First(&actual, 29).Error; err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, f.existing) {
			t.Fatalf("failed update changed binding: before=%+v after=%+v", f.existing, actual)
		}
	} else {
		var count int64
		if err := f.db.Model(&model.ChannelTemplateBinding{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("failed create persisted %d bindings", count)
		}
	}
}

func TestChannelBindingCanRetryAfterApproval(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(fmt.Sprintf("update=%t", update), func(t *testing.T) {
			f := newBindingControllerFixture(t, update)
			if err := f.db.Model(&model.ProviderTemplate{}).Where("id = ?", 25).Update("audit_status", 1).Error; err != nil {
				t.Fatal(err)
			}
			ctx := f.request(t, update, nil)
			f.assertUnchanged(t, update)
			if ctx.status != 400 || ctx.body.Message != pendingTemplateMessage {
				t.Fatalf("pending response: %+v", ctx.body)
			}
			if err := f.db.Model(&model.ProviderTemplate{}).Where("id = ?", 25).Update("audit_status", 2).Error; err != nil {
				t.Fatal(err)
			}
			ctx = f.request(t, update, nil)
			if ctx.status != 200 || ctx.body.Code != 0 {
				t.Fatalf("approved retry: %+v", ctx.body)
			}
		})
	}
}

func TestChannelBindingDatabaseFailureRemainsInternalError(t *testing.T) {
	for _, update := range []bool{false, true} {
		t.Run(fmt.Sprintf("update=%t", update), func(t *testing.T) {
			f := newBindingControllerFixture(t, update)
			sqlDB, _ := f.db.DB()
			if err := sqlDB.Close(); err != nil {
				t.Fatal(err)
			}
			ctx := f.request(t, update, nil)
			if ctx.status != 500 || ctx.body.Code != 500 || !strings.Contains(ctx.body.Message, "database is closed") {
				t.Fatalf("database error response: %d %+v", ctx.status, ctx.body)
			}
		})
	}
}

func TestChannelBindingWrappedValidationErrorReturnsBadRequest(t *testing.T) {
	ctx := &bindingControllerContext{}
	err := fmt.Errorf("wrapped: %w", &service.ChannelBindingValidationError{
		Codes: []string{"PROVIDER_TEMPLATE_UNAVAILABLE"}, Message: pendingTemplateMessage,
	})
	writeChannelBindingError(ctx, err, "failed to update channel binding")
	if ctx.status != 400 || ctx.body.Code != 400 || ctx.body.Message != pendingTemplateMessage {
		t.Fatalf("wrapped validation error response: %d %+v", ctx.status, ctx.body)
	}
}
