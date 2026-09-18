package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/readiness"
	"cnb.cool/mliev/push/message-push/modules/channel/domain"
	registry "cnb.cool/mliev/push/message-push/modules/sender/domain"
	senderinfra "cnb.cool/mliev/push/message-push/modules/sender/infrastructure"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type catalogFixture struct {
	channel  *model.Channel
	template *model.MessageTemplate
	account  *model.ProviderAccount
	binding  *model.ChannelTemplateBinding
}

func TestQQCatalogUsesExistingTemplateAndReceiverContract(t *testing.T) {
	db := newCatalogTestDB(t)
	f := createCatalogFixture(t, db, constants.ProviderOneBot, constants.MessageTypeQQ)
	s := NewCatalogService(db)
	list, err := s.ListChannels(context.Background(), dto.PublicChannelListRequest{Type: constants.MessageTypeQQ, Page: 1, PageSize: 20})
	if err != nil || len(list.Items) != 1 || list.Items[0].Type != constants.MessageTypeQQ {
		t.Fatalf("QQ list=%+v err=%v", list, err)
	}
	detail, err := s.GetChannel(context.Background(), f.channel.ID)
	if err != nil || detail.SignatureRequired || detail.Readiness.State != constants.ChannelReadinessReady {
		t.Fatalf("QQ detail=%+v err=%v", detail, err)
	}
	if len(detail.SignatureNames) != 0 {
		t.Fatal("unconfigured QQ channel exposes signature choices")
	}
	addCatalogAlias(t, db, f.channel.ID, f.account.ID, "notice")
	detail, err = s.GetChannel(context.Background(), f.channel.ID)
	if err != nil || detail.SignatureRequired || !reflect.DeepEqual(detail.SignatureNames, []string{"notice"}) {
		t.Fatalf("optional QQ aliases=%+v err=%v", detail, err)
	}
	second, _ := addCatalogProvider(t, db, f.channel, constants.ProviderOneBot)
	detail, err = s.GetChannel(context.Background(), f.channel.ID)
	if err != nil || detail.SignatureRequired || len(detail.SignatureNames) != 0 {
		t.Fatalf("unshared QQ alias=%+v err=%v", detail, err)
	}
	addCatalogAlias(t, db, f.channel.ID, second.ID, "notice")
	detail, err = s.GetChannel(context.Background(), f.channel.ID)
	if err != nil || detail.SignatureRequired || !reflect.DeepEqual(detail.SignatureNames, []string{"notice"}) {
		t.Fatalf("shared QQ aliases=%+v err=%v", detail, err)
	}
}

func newCatalogTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newSelectorReadinessDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func createCatalogFixture(t *testing.T, db *gorm.DB, provider, channelType string) catalogFixture {
	t.Helper()
	template := &model.MessageTemplate{TemplateName: "验证码", Content: "您的验证码是{code}，{expire}分钟有效。", ContentType: "text", Variables: `["code","expire"]`, Description: "登录验证码", Status: 1}
	insertCatalog(t, db, template)
	channel := &model.Channel{Name: "验证通道", Type: channelType, MessageTemplateID: template.ID, Status: 1}
	insertCatalog(t, db, channel)
	account, binding := addCatalogProvider(t, db, channel, provider)
	return catalogFixture{channel: channel, template: template, account: account, binding: binding}
}

func addCatalogProvider(t *testing.T, db *gorm.DB, channel *model.Channel, provider string) (*model.ProviderAccount, *model.ChannelTemplateBinding) {
	t.Helper()
	account := &model.ProviderAccount{AccountCode: uuid.NewString(), AccountName: "internal-account", ProviderCode: provider, ProviderType: channel.Type, Status: 1, Config: `{"secret":"private-account-secret"}`}
	insertCatalog(t, db, account)
	template := &model.ProviderTemplate{TemplateContent: "code={code}", ContentVersion: 1, ProviderID: account.ID, TemplateCode: "private-template-code", TemplateName: "provider-template", Variables: `["code"]`, Status: 1}
	if provider == constants.ProviderAliyunSMS {
		template.TemplateContent = "code=${code}"
	}
	if provider == constants.ProviderTencentSMS {
		template.TemplateContent = "code={1}"
	}
	insertCatalog(t, db, template)
	binding := &model.ChannelTemplateBinding{MappedContentVersion: 1, ParamMapping: `[{"type":"mapping","provider_var":"code","system_var":"code"}]`, ChannelID: channel.ID, ProviderID: account.ID, ProviderTemplateID: template.ID, Weight: 10, Priority: 100, Status: 1, IsActive: 1}
	if provider == constants.ProviderTencentSMS {
		binding.ParamMapping = `[{"type":"mapping","provider_var":"1","system_var":"code"}]`
	}
	insertCatalog(t, db, binding)
	return account, binding
}

func addCatalogAlias(t *testing.T, db *gorm.DB, channelID, accountID uint, alias string) *model.ChannelSignatureMapping {
	t.Helper()
	signature := &model.ProviderSignature{ProviderAccountID: accountID, SignatureName: "private-signature-name", SignatureCode: "private-signature-code", Status: 1}
	insertCatalog(t, db, signature)
	mapping := &model.ChannelSignatureMapping{ChannelID: channelID, ProviderID: accountID, ProviderSignatureID: signature.ID, SignatureName: alias, Status: 1}
	insertCatalog(t, db, mapping)
	return mapping
}

func insertCatalog(t *testing.T, db *gorm.DB, value any) {
	t.Helper()
	if err := db.Create(value).Error; err != nil {
		t.Fatal(err)
	}
}

func updateCatalog(t *testing.T, db *gorm.DB, value any, field string, replacement any) {
	t.Helper()
	if err := db.Model(value).Update(field, replacement).Error; err != nil {
		t.Fatal(err)
	}
}

func TestCatalogVisibilityPaginationAndBatchQueries(t *testing.T) {
	db := newCatalogTestDB(t)
	s := NewCatalogService(db)
	empty, err := s.ListChannels(context.Background(), dto.PublicChannelListRequest{Page: 1, PageSize: 20})
	if err != nil || empty.Total != 0 || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("empty catalog = %+v, err=%v", empty, err)
	}
	sms := createCatalogFixture(t, db, constants.ProviderAliyunSMS, constants.MessageTypeSMS)
	email := createCatalogFixture(t, db, constants.ProviderSMTP, constants.MessageTypeEmail)
	disabled := createCatalogFixture(t, db, constants.ProviderAliyunSMS, constants.MessageTypeSMS)
	deleted := createCatalogFixture(t, db, constants.ProviderAliyunSMS, constants.MessageTypeSMS)
	updateCatalog(t, db, disabled.channel, "status", 0)
	if err := db.Delete(deleted.channel).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint{disabled.channel.ID, deleted.channel.ID, 99999} {
		if _, err := s.GetChannel(context.Background(), id); !errors.Is(err, domain.ErrCatalogChannelNotFound) {
			t.Fatalf("hidden channel %d: %v", id, err)
		}
	}
	queries := 0
	if err := db.Callback().Query().Before("gorm:query").Register("catalog_query_count", func(*gorm.DB) { queries++ }); err != nil {
		t.Fatal(err)
	}
	first, err := s.ListChannels(context.Background(), dto.PublicChannelListRequest{Page: 1, PageSize: 1})
	if err != nil || first.Total != 2 || len(first.Items) != 1 || first.Items[0].ID != email.channel.ID || first.Size != 1 || first.Page != 1 {
		t.Fatalf("first page = %+v, err=%v", first, err)
	}
	singleQueries := queries
	queries = 0
	all, err := s.ListChannels(context.Background(), dto.PublicChannelListRequest{Page: 1, PageSize: 100})
	if err != nil || len(all.Items) != 2 || all.Items[1].ID != sms.channel.ID {
		t.Fatalf("all channels = %+v, err=%v", all, err)
	}
	if queries != singleQueries {
		t.Fatalf("query count grew with page size: one=%d, all=%d", singleQueries, queries)
	}
	if all.Items[0].Readiness.State != constants.ChannelReadinessBlocked {
		t.Fatalf("enabled blocked channel was hidden or marked ready: %+v", all.Items[0])
	}
	filtered, err := s.ListChannels(context.Background(), dto.PublicChannelListRequest{Type: "sms", Page: 1, PageSize: 20})
	if err != nil || filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].ID != sms.channel.ID {
		t.Fatalf("filtered = %+v, err=%v", filtered, err)
	}
	pastEnd, err := s.ListChannels(context.Background(), dto.PublicChannelListRequest{Page: 3, PageSize: 1})
	if err != nil || pastEnd.Total != 2 || pastEnd.Items == nil || len(pastEnd.Items) != 0 {
		t.Fatalf("past last page = %+v, err=%v", pastEnd, err)
	}
	encoded, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	for _, internal := range []string{"private-", "provider_id", "content", "signature_names", "blockers\""} {
		if strings.Contains(string(encoded), internal) {
			t.Errorf("catalog list leaked %q: %s", internal, encoded)
		}
	}
}

func TestCatalogTemplateProjection(t *testing.T) {
	type templateCase struct {
		name       string
		mutate     func(*testing.T, *gorm.DB, catalogFixture)
		wantType   string
		wantVars   []string
		wantCode   string
		wantAbsent bool
	}
	tests := []templateCase{
		{name: "text and variable order", wantType: "text", wantVars: []string{"code", "expire"}},
		{name: "empty content type", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture) {
			updateCatalog(t, db, f.template, "content_type", "")
		}, wantType: "text", wantVars: []string{"code", "expire"}},
		{name: "html", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture) {
			updateCatalog(t, db, f.template, "content_type", "html")
		}, wantType: "html", wantVars: []string{"code", "expire"}},
		{name: "markdown", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture) {
			updateCatalog(t, db, f.template, "content_type", "markdown")
		}, wantType: "markdown", wantVars: []string{"code", "expire"}},
		{name: "no variables", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture) {
			updateCatalog(t, db, f.template, "variables", "")
			updateCatalog(t, db, &model.ProviderTemplate{ID: f.binding.ProviderTemplateID}, "template_content", "静态通知")
			updateCatalog(t, db, f.binding, "param_mapping", "[]")
		}, wantType: "text", wantVars: []string{}},
		{name: "missing", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture) {
			updateCatalog(t, db, f.channel, "message_template_id", 99999)
		}, wantCode: constants.ReadinessBlockerMessageTemplateMissing, wantAbsent: true},
		{name: "deleted", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture) {
			if err := db.Delete(f.template).Error; err != nil {
				t.Fatal(err)
			}
		}, wantCode: constants.ReadinessBlockerMessageTemplateMissing, wantAbsent: true},
		{name: "disabled", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture) { updateCatalog(t, db, f.template, "status", 0) }, wantType: "text", wantVars: []string{"code", "expire"}, wantCode: constants.ReadinessBlockerMessageTemplateDisabled},
	}
	for _, invalid := range []string{`broken`, `[1]`, `["code","code"]`, `[" code"]`, `[""]`} {
		tests = append(tests, templateCase{name: "invalid " + invalid, mutate: func(t *testing.T, db *gorm.DB, f catalogFixture) {
			updateCatalog(t, db, f.template, "variables", invalid)
		}, wantType: "text", wantCode: constants.ReadinessBlockerMessageTemplateVariablesInvalid})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newCatalogTestDB(t)
			f := createCatalogFixture(t, db, constants.ProviderAliyunSMS, "sms")
			addCatalogAlias(t, db, f.channel.ID, f.account.ID, "验证码")
			if tt.mutate != nil {
				tt.mutate(t, db, f)
			}
			result, err := NewCatalogService(db).GetChannel(context.Background(), f.channel.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tt.wantCode != "" && (result.Readiness.State != "blocked" || !slices.Contains(result.Readiness.BlockerCodes, tt.wantCode)) {
				t.Fatalf("readiness = %+v, want blocked %s", result.Readiness, tt.wantCode)
			}
			if tt.wantCode == "" && result.Readiness.State != "ready" {
				t.Fatalf("readiness = %+v", result.Readiness)
			}
			if tt.wantAbsent {
				if result.Template != nil {
					t.Fatalf("missing template = %+v", result.Template)
				}
			} else if result.Template == nil || result.Template.ContentType != tt.wantType || !reflect.DeepEqual(result.Template.Variables, tt.wantVars) || result.Template.Content != f.template.Content || result.Template.Description != f.template.Description {
				t.Fatalf("template = %+v; want type=%s vars=%#v", result.Template, tt.wantType, tt.wantVars)
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			for _, internal := range []string{"private-", "provider_id", "provider_account", "signature_code", "created_at", "deleted_at"} {
				if strings.Contains(string(encoded), internal) {
					t.Errorf("detail leaked %q: %s", internal, encoded)
				}
			}
			if tt.wantCode != "" && (len(result.SignatureNames) != 0 || result.SignatureRequired) {
				t.Fatalf("blocked signature options = %+v", result)
			}
		})
	}
}

func TestCatalogSignatureOptionsMatchSendValidation(t *testing.T) {
	const plainProvider = "catalog_test_sms_plain"
	if !registry.GetRegistry().Exists(plainProvider) {
		if err := registry.Register(&registry.ProviderMeta{Code: plainProvider, TemplateCodec: senderinfra.NativeTemplateCodec{ID: "fixture-native", AllowNamed: true, AllowNumeric: true}, Name: "plain test provider", Type: "sms"}); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name      string
		mutate    func(*testing.T, *gorm.DB, catalogFixture, *model.ProviderAccount, *model.ChannelTemplateBinding, *model.ChannelSignatureMapping)
		wantState string
		wantNames []string
	}{
		{name: "common aliases deduplicated and sorted", wantState: "ready", wantNames: []string{"alpha", "zeta"}},
		{name: "disabled mapping", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture, a *model.ProviderAccount, b *model.ChannelTemplateBinding, m *model.ChannelSignatureMapping) {
			updateCatalog(t, db, m, "status", 0)
		}, wantState: "ready", wantNames: []string{"zeta"}},
		{name: "disabled signature", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture, a *model.ProviderAccount, b *model.ChannelTemplateBinding, m *model.ChannelSignatureMapping) {
			updateCatalog(t, db, &model.ProviderSignature{ID: m.ProviderSignatureID}, "status", 0)
		}, wantState: "ready", wantNames: []string{"zeta"}},
		{name: "deleted signature", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture, a *model.ProviderAccount, b *model.ChannelTemplateBinding, m *model.ChannelSignatureMapping) {
			if err := db.Delete(&model.ProviderSignature{}, m.ProviderSignatureID).Error; err != nil {
				t.Fatal(err)
			}
		}, wantState: "ready", wantNames: []string{"zeta"}},
		{name: "inactive provider binding", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture, a *model.ProviderAccount, b *model.ChannelTemplateBinding, m *model.ChannelSignatureMapping) {
			updateCatalog(t, db, b, "is_active", 0)
		}, wantState: "degraded", wantNames: []string{"alpha", "only-first", "zeta"}},
		{name: "no common alias", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture, a *model.ProviderAccount, b *model.ChannelTemplateBinding, m *model.ChannelSignatureMapping) {
			if err := db.Model(&model.ChannelSignatureMapping{}).Where("provider_id = ?", a.ID).Update("status", 0).Error; err != nil {
				t.Fatal(err)
			}
			addCatalogAlias(t, db, f.channel.ID, a.ID, "different")
		}, wantState: "blocked", wantNames: []string{}},
		{name: "optional path survives missing required aliases", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture, a *model.ProviderAccount, b *model.ChannelTemplateBinding, m *model.ChannelSignatureMapping) {
			if err := db.Model(&model.ChannelSignatureMapping{}).Where("provider_id = ?", a.ID).Update("status", 0).Error; err != nil {
				t.Fatal(err)
			}
			addCatalogProvider(t, db, f.channel, plainProvider)
		}, wantState: "degraded", wantNames: []string{}},
		{name: "no signature required", mutate: func(t *testing.T, db *gorm.DB, f catalogFixture, a *model.ProviderAccount, b *model.ChannelTemplateBinding, m *model.ChannelSignatureMapping) {
			updateCatalog(t, db, a, "provider_code", plainProvider)
			updateCatalog(t, db, f.account, "provider_code", plainProvider)
		}, wantState: "ready", wantNames: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newCatalogTestDB(t)
			f := createCatalogFixture(t, db, constants.ProviderAliyunSMS, "sms")
			a, b := addCatalogProvider(t, db, f.channel, constants.ProviderTencentSMS)
			for _, alias := range []string{"zeta", "alpha", "only-first"} {
				addCatalogAlias(t, db, f.channel.ID, f.account.ID, alias)
			}
			addCatalogAlias(t, db, f.channel.ID, a.ID, "zeta")
			m := addCatalogAlias(t, db, f.channel.ID, a.ID, "alpha")
			if tt.mutate != nil {
				tt.mutate(t, db, f, a, b, m)
			}
			result, err := NewCatalogService(db).GetChannel(context.Background(), f.channel.ID)
			if err != nil {
				t.Fatal(err)
			}
			if result.Readiness.State != tt.wantState || !reflect.DeepEqual(result.SignatureNames, tt.wantNames) || result.SignatureRequired != (len(tt.wantNames) > 0) {
				t.Fatalf("catalog = %+v, want %s %v", result, tt.wantState, tt.wantNames)
			}
			evaluator := readiness.NewChannelEvaluator(db)
			for _, alias := range result.SignatureNames {
				if err := evaluator.ValidateForSend(f.channel.ID, alias); err != nil {
					t.Errorf("offered alias %q rejected: %v", alias, err)
				}
			}
			emptyErr := evaluator.ValidateForSend(f.channel.ID, "")
			if (emptyErr != nil) != (result.SignatureRequired || result.Readiness.State == "blocked") {
				t.Fatalf("empty alias validation disagrees: %v, %+v", emptyErr, result)
			}
			if result.SignatureRequired && evaluator.ValidateForSend(f.channel.ID, "unknown") == nil {
				t.Fatal("unknown alias accepted")
			}
		})
	}
}

func TestCatalogSMTPTitleAndRobotOptions(t *testing.T) {
	for _, test := range []struct {
		provider    string
		channelType string
		required    bool
	}{
		{constants.ProviderSMTP, constants.MessageTypeEmail, true},
		{constants.ProviderWeChatWorkRobot, constants.MessageTypeWeChatWork, false},
		{constants.ProviderDingTalkRobot, constants.MessageTypeDingTalk, false},
	} {
		t.Run(test.provider, func(t *testing.T) {
			db := newCatalogTestDB(t)
			f := createCatalogFixture(t, db, test.provider, test.channelType)
			addCatalogAlias(t, db, f.channel.ID, f.account.ID, "通知标题")
			result, err := NewCatalogService(db).GetChannel(context.Background(), f.channel.ID)
			if err != nil {
				t.Fatal(err)
			}
			wantNames := []string{}
			if test.required {
				wantNames = []string{"通知标题"}
			}
			if result.Readiness.State != "ready" || result.SignatureRequired != test.required || !reflect.DeepEqual(result.SignatureNames, wantNames) {
				t.Fatalf("unexpected options: %+v", result)
			}
		})
	}
}

func TestCatalogRejectsInvalidQueriesAndPropagatesContext(t *testing.T) {
	db := newCatalogTestDB(t)
	s := NewCatalogService(db)
	for _, req := range []dto.PublicChannelListRequest{{Page: 0, PageSize: 20}, {Page: 1, PageSize: 101}, {Page: -1, PageSize: 20}, {Page: 1, PageSize: 0}, {Page: 1, PageSize: 20, Type: "unknown"}, {Page: int(^uint(0) >> 1), PageSize: 100}} {
		if _, err := s.ListChannels(context.Background(), req); !errors.Is(err, domain.ErrInvalidCatalogRequest) {
			t.Errorf("request %+v: %v", req, err)
		}
	}
	if _, err := s.GetChannel(context.Background(), 0); !errors.Is(err, domain.ErrInvalidCatalogRequest) {
		t.Fatalf("zero ID: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.ListChannels(ctx, dto.PublicChannelListRequest{Page: 1, PageSize: 20}); !errors.Is(err, context.Canceled) {
		t.Errorf("list cancellation: %v", err)
	}
	if _, err := s.GetChannel(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Errorf("detail cancellation: %v", err)
	}
	// A canceled context must reach the association/readiness queries too.
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	f := createCatalogFixture(t, db, constants.ProviderAliyunSMS, "sms")
	if err := db.Callback().Query().Before("gorm:query").Register("catalog_cancel_readiness", func(tx *gorm.DB) {
		if tx.Statement.Table == "channel_template_bindings" {
			cancel()
		}
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetChannel(ctx, f.channel.ID); !errors.Is(err, context.Canceled) {
		t.Errorf("readiness cancellation: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetChannel(context.Background(), f.channel.ID); err == nil || errors.Is(err, domain.ErrCatalogChannelNotFound) {
		t.Fatalf("database failure misclassified: %v", err)
	}
}
