package readiness

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	registry "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const (
	testProviderSMSPlain      = "readiness_test_sms_plain"
	testProviderSMSRequired   = "readiness_test_sms_required"
	testProviderEmailPlain    = "readiness_test_email_plain"
	testProviderWechatPlain   = "readiness_test_wechat_plain"
	testProviderDingTalkPlain = "readiness_test_dingtalk_plain"
)

func TestChannelEvaluatorStatesAndBindingValidation(t *testing.T) {
	registerReadinessTestProviders(t)

	tests := []struct {
		name         string
		messageType  string
		providerCode string
		mutate       func(t *testing.T, db *gorm.DB, fixture *readinessFixture)
		wantState    string
		wantValid    int
		wantCode     string
	}{
		{name: "sms ready", messageType: constants.MessageTypeSMS, providerCode: testProviderSMSPlain, wantState: constants.ChannelReadinessReady, wantValid: 1},
		{name: "email ready", messageType: constants.MessageTypeEmail, providerCode: testProviderEmailPlain, wantState: constants.ChannelReadinessReady, wantValid: 1},
		{name: "wechat ready", messageType: constants.MessageTypeWeChatWork, providerCode: testProviderWechatPlain, wantState: constants.ChannelReadinessReady, wantValid: 1},
		{name: "dingtalk ready", messageType: constants.MessageTypeDingTalk, providerCode: testProviderDingTalkPlain, wantState: constants.ChannelReadinessReady, wantValid: 1},
		{
			name:         "disabled spare binding is ignored",
			messageType:  constants.MessageTypeSMS,
			providerCode: testProviderSMSPlain,
			mutate: func(t *testing.T, db *gorm.DB, fixture *readinessFixture) {
				spare := createBinding(t, db, fixture.channel.ID, fixture.providerTemplate.ID, fixture.account.ID, 1, 1, "")
				if err := db.Model(&model.ChannelTemplateBinding{}).Where("id = ?", spare.ID).Update("status", 0).Error; err != nil {
					t.Fatal(err)
				}
			},
			wantState: constants.ChannelReadinessReady,
			wantValid: 1,
		},
		{
			name:         "auto disabled spare binding degrades",
			messageType:  constants.MessageTypeSMS,
			providerCode: testProviderSMSPlain,
			mutate: func(t *testing.T, db *gorm.DB, fixture *readinessFixture) {
				spare := createBinding(t, db, fixture.channel.ID, fixture.providerTemplate.ID, fixture.account.ID, 1, 1, "")
				if err := db.Model(&model.ChannelTemplateBinding{}).Where("id = ?", spare.ID).Update("is_active", 0).Error; err != nil {
					t.Fatal(err)
				}
			},
			wantState: constants.ChannelReadinessDegraded,
			wantValid: 1,
			wantCode:  constants.ReadinessBlockerBindingInactive,
		},
		{
			name:         "channel disabled blocks",
			messageType:  constants.MessageTypeSMS,
			providerCode: testProviderSMSPlain,
			mutate: func(t *testing.T, db *gorm.DB, fixture *readinessFixture) {
				if err := db.Model(&model.Channel{}).Where("id = ?", fixture.channel.ID).Update("status", 0).Error; err != nil {
					t.Fatal(err)
				}
			},
			wantState: constants.ChannelReadinessBlocked,
			wantValid: 0,
			wantCode:  constants.ReadinessBlockerChannelDisabled,
		},
		{
			name:         "disabled provider account blocks",
			messageType:  constants.MessageTypeSMS,
			providerCode: testProviderSMSPlain,
			mutate: func(t *testing.T, db *gorm.DB, fixture *readinessFixture) {
				if err := db.Model(&model.ProviderAccount{}).Where("id = ?", fixture.account.ID).Update("status", 0).Error; err != nil {
					t.Fatal(err)
				}
			},
			wantState: constants.ChannelReadinessBlocked,
			wantValid: 0,
			wantCode:  constants.ReadinessBlockerProviderAccountDisabled,
		},
		{
			name:         "orphan binding blocks",
			messageType:  constants.MessageTypeSMS,
			providerCode: testProviderSMSPlain,
			mutate: func(t *testing.T, db *gorm.DB, fixture *readinessFixture) {
				if err := db.Delete(&model.ChannelTemplateBinding{}, fixture.binding.ID).Error; err != nil {
					t.Fatal(err)
				}
				createBinding(t, db, fixture.channel.ID, 9999, 9999, 1, 1, "")
			},
			wantState: constants.ChannelReadinessBlocked,
			wantValid: 0,
			wantCode:  constants.ReadinessBlockerProviderTemplateMissing,
		},
		{
			name:         "invalid mapping blocks",
			messageType:  constants.MessageTypeSMS,
			providerCode: testProviderSMSPlain,
			mutate: func(t *testing.T, db *gorm.DB, fixture *readinessFixture) {
				invalid := `[{"type":"mapping","provider_var":"code","system_var":"missing","value":""}]`
				if err := db.Model(&model.ChannelTemplateBinding{}).Where("id = ?", fixture.binding.ID).Update("param_mapping", invalid).Error; err != nil {
					t.Fatal(err)
				}
			},
			wantState: constants.ChannelReadinessBlocked,
			wantValid: 0,
			wantCode:  constants.ReadinessBlockerParamMappingInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newReadinessTestDB(t)
			fixture := createReadyFixture(t, db, tt.messageType, tt.providerCode)
			if tt.mutate != nil {
				tt.mutate(t, db, fixture)
			}

			result, err := NewChannelEvaluator(db).EvaluateChannel(fixture.channel.ID)
			if err != nil {
				t.Fatalf("evaluate channel: %v", err)
			}
			if result.State != tt.wantState || result.ValidBindingCount != tt.wantValid {
				t.Fatalf("readiness = state=%s valid=%d blockers=%v, want state=%s valid=%d", result.State, result.ValidBindingCount, result.BlockerCodes, tt.wantState, tt.wantValid)
			}
			if tt.wantCode != "" && !containsString(result.BlockerCodes, tt.wantCode) {
				t.Fatalf("blockers %v do not contain %s", result.BlockerCodes, tt.wantCode)
			}
			if tt.wantState == constants.ChannelReadinessBlocked {
				eligibility, err := NewChannelEvaluator(db).GetDeliveryEligibility(fixture.channel.ID)
				if err != nil {
					t.Fatal(err)
				}
				if len(eligibility.ValidBindingIDs) != 0 {
					t.Fatalf("blocked channel leaked selector candidates: %v", eligibility.ValidBindingIDs)
				}
			}
		})
	}
}

func TestChannelEvaluatorEmptyChannelIsBlocked(t *testing.T) {
	registerReadinessTestProviders(t)
	db := newReadinessTestDB(t)
	template := createMessageTemplate(t, db)
	channel := &model.Channel{Name: "empty", Type: constants.MessageTypeSMS, MessageTemplateID: template.ID, Status: 1}
	if err := db.Create(channel).Error; err != nil {
		t.Fatal(err)
	}

	result, err := NewChannelEvaluator(db).EvaluateChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != constants.ChannelReadinessBlocked || !containsString(result.BlockerCodes, constants.ReadinessBlockerNoBindings) {
		t.Fatalf("unexpected empty readiness: %+v", result)
	}
}

func TestRequiredSignatureCommonAliasControlsStaticEligibility(t *testing.T) {
	registerReadinessTestProviders(t)
	db := newReadinessTestDB(t)
	template := createMessageTemplate(t, db)
	channel := &model.Channel{Name: "signature", Type: constants.MessageTypeSMS, MessageTemplateID: template.ID, Status: 1}
	if err := db.Create(channel).Error; err != nil {
		t.Fatal(err)
	}

	requiredA, templateA, bindingA := createProviderPath(t, db, channel.ID, constants.MessageTypeSMS, testProviderSMSRequired)
	requiredB, templateB, bindingB := createProviderPath(t, db, channel.ID, constants.MessageTypeSMS, testProviderSMSRequired)
	_ = templateA
	_ = templateB
	createSignatureAlias(t, db, channel.ID, requiredA.ID, "alpha")
	mappingB := createSignatureAlias(t, db, channel.ID, requiredB.ID, "beta")

	evaluator := NewChannelEvaluator(db)
	result, err := evaluator.EvaluateChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != constants.ChannelReadinessBlocked || result.ValidBindingCount != 0 || !containsString(result.BlockerCodes, constants.ReadinessBlockerSignatureAliasNotCommon) {
		t.Fatalf("disjoint required aliases should block: %+v", result)
	}
	eligibility, err := evaluator.GetDeliveryEligibility(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(eligibility.ValidBindingIDs) != 0 {
		t.Fatalf("selector eligibility must exclude all disjoint required bindings, got %v (bindings %d,%d)", eligibility.ValidBindingIDs, bindingA.ID, bindingB.ID)
	}
	if len(result.CommonSignatureAliases) != 0 || len(eligibility.CommonSignatureAliases) != 0 {
		t.Fatalf("disjoint aliases must expose no common alias: readiness=%v eligibility=%v", result.CommonSignatureAliases, eligibility.CommonSignatureAliases)
	}

	plain, _, plainBinding := createProviderPath(t, db, channel.ID, constants.MessageTypeSMS, testProviderSMSPlain)
	_ = plain
	result, err = evaluator.EvaluateChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != constants.ChannelReadinessDegraded || result.ValidBindingCount != 1 {
		t.Fatalf("plain fallback should degrade, got %+v", result)
	}
	eligibility, err = evaluator.GetDeliveryEligibility(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(eligibility.ValidBindingIDs) != 1 || eligibility.ValidBindingIDs[0] != plainBinding.ID {
		t.Fatalf("selector eligibility = %v, want only plain binding %d", eligibility.ValidBindingIDs, plainBinding.ID)
	}
	if err := evaluator.ValidateForSend(channel.ID, ""); err != nil {
		t.Fatalf("plain fallback should not require alias: %v", err)
	}

	if err := db.Model(&model.ChannelSignatureMapping{}).Where("id = ?", mappingB.ID).Update("signature_name", "alpha").Error; err != nil {
		t.Fatal(err)
	}
	result, err = evaluator.EvaluateChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != constants.ChannelReadinessReady || result.ValidBindingCount != 3 || result.ConfiguredSignatureAliasCount != 1 {
		t.Fatalf("shared alias should enable all paths: %+v", result)
	}
	if len(result.CommonSignatureAliases) != 1 || result.CommonSignatureAliases[0] != "alpha" {
		t.Fatalf("common aliases = %v, want [alpha]", result.CommonSignatureAliases)
	}
	eligibility, err = evaluator.GetDeliveryEligibility(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(eligibility.CommonSignatureAliases) != 1 || eligibility.CommonSignatureAliases[0] != "alpha" {
		t.Fatalf("eligibility aliases = %v, want [alpha]", eligibility.CommonSignatureAliases)
	}
	if err := evaluator.ValidateForSend(channel.ID, "alpha"); err != nil {
		t.Fatalf("shared alias should validate: %v", err)
	}
	if err := evaluator.ValidateForSend(channel.ID, ""); !validationHasCode(err, constants.ReadinessBlockerSignatureRequired) {
		t.Fatalf("empty alias error = %v, want %s", err, constants.ReadinessBlockerSignatureRequired)
	}
	if err := evaluator.ValidateForSend(channel.ID, "beta"); !validationHasCode(err, constants.ReadinessBlockerSignatureAliasNotCommon) {
		t.Fatalf("non-common alias error = %v, want %s", err, constants.ReadinessBlockerSignatureAliasNotCommon)
	}

	if err := db.Create(&model.ChannelSignatureMapping{
		ChannelID:           channel.ID,
		SignatureName:       "stale",
		ProviderSignatureID: 999999,
		ProviderID:          requiredA.ID,
		Status:              1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	result, err = evaluator.EvaluateChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != constants.ChannelReadinessReady || result.ValidBindingCount != 3 {
		t.Fatalf("redundant invalid signature mapping must not degrade valid routes: %+v", result)
	}
}

func TestMissingRequiredSignatureDegradesWhenPlainPathRemains(t *testing.T) {
	registerReadinessTestProviders(t)
	db := newReadinessTestDB(t)
	template := createMessageTemplate(t, db)
	channel := &model.Channel{Name: "mixed", Type: constants.MessageTypeSMS, MessageTemplateID: template.ID, Status: 1}
	if err := db.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	createProviderPath(t, db, channel.ID, constants.MessageTypeSMS, testProviderSMSRequired)
	_, _, plainBinding := createProviderPath(t, db, channel.ID, constants.MessageTypeSMS, testProviderSMSPlain)

	evaluator := NewChannelEvaluator(db)
	result, err := evaluator.EvaluateChannel(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != constants.ChannelReadinessDegraded || result.ValidBindingCount != 1 || !containsString(result.BlockerCodes, constants.ReadinessBlockerSignatureRequired) {
		t.Fatalf("unexpected mixed readiness: %+v", result)
	}
	eligibility, err := evaluator.GetDeliveryEligibility(channel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(eligibility.ValidBindingIDs) != 1 || eligibility.ValidBindingIDs[0] != plainBinding.ID {
		t.Fatalf("eligibility = %v, want plain binding %d", eligibility.ValidBindingIDs, plainBinding.ID)
	}
}

type readinessFixture struct {
	channel          *model.Channel
	account          *model.ProviderAccount
	providerTemplate *model.ProviderTemplate
	binding          *model.ChannelTemplateBinding
}

func newReadinessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "readiness.db")), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE message_templates (id INTEGER PRIMARY KEY AUTOINCREMENT, template_name TEXT NOT NULL, content_type TEXT, content TEXT, variables TEXT, description TEXT, status INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE channels (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, type TEXT NOT NULL, message_template_id INTEGER, status INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_accounts (id INTEGER PRIMARY KEY AUTOINCREMENT, account_code TEXT NOT NULL UNIQUE, account_name TEXT NOT NULL, provider_code TEXT NOT NULL, provider_type TEXT NOT NULL, config TEXT, status INTEGER, remark TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_templates (id INTEGER PRIMARY KEY AUTOINCREMENT, provider_id INTEGER NOT NULL, template_code TEXT NOT NULL, template_name TEXT NOT NULL, content_type TEXT, template_content TEXT, variables TEXT, status INTEGER, remark TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE channel_template_bindings (id INTEGER PRIMARY KEY AUTOINCREMENT, channel_id INTEGER NOT NULL, provider_template_id INTEGER NOT NULL, provider_id INTEGER NOT NULL, param_mapping TEXT, weight INTEGER, priority INTEGER, status INTEGER, is_active INTEGER, auto_disable_on_fail INTEGER, auto_disable_threshold INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_signatures (id INTEGER PRIMARY KEY AUTOINCREMENT, provider_account_id INTEGER NOT NULL, signature_code TEXT NOT NULL, signature_name TEXT NOT NULL, status INTEGER, remark TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE channel_signature_mappings (id INTEGER PRIMARY KEY AUTOINCREMENT, channel_id INTEGER NOT NULL, signature_name TEXT NOT NULL, provider_signature_id INTEGER NOT NULL, provider_id INTEGER NOT NULL, status INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create readiness schema: %v", err)
		}
	}
	return db
}

func registerReadinessTestProviders(t *testing.T) {
	t.Helper()
	providers := []*registry.ProviderMeta{
		{Code: testProviderSMSPlain, Name: "SMS plain", Type: constants.MessageTypeSMS},
		{Code: testProviderSMSRequired, Name: "SMS required", Type: constants.MessageTypeSMS, RequiresSignature: true},
		{Code: testProviderEmailPlain, Name: "Email plain", Type: constants.MessageTypeEmail},
		{Code: testProviderWechatPlain, Name: "Wechat plain", Type: constants.MessageTypeWeChatWork},
		{Code: testProviderDingTalkPlain, Name: "DingTalk plain", Type: constants.MessageTypeDingTalk},
	}
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil && !stringsContains(err.Error(), "already registered") {
			t.Fatalf("register provider %s: %v", provider.Code, err)
		}
	}
}

func createReadyFixture(t *testing.T, db *gorm.DB, messageType, providerCode string) *readinessFixture {
	t.Helper()
	template := createMessageTemplate(t, db)
	channel := &model.Channel{Name: messageType, Type: messageType, MessageTemplateID: template.ID, Status: 1}
	if err := db.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	account, providerTemplate, binding := createProviderPath(t, db, channel.ID, messageType, providerCode)
	return &readinessFixture{channel: channel, account: account, providerTemplate: providerTemplate, binding: binding}
}

func createMessageTemplate(t *testing.T, db *gorm.DB) *model.MessageTemplate {
	t.Helper()
	template := &model.MessageTemplate{TemplateName: "system", Content: "code={code}", Status: 1}
	if err := template.SetVariables([]string{"code"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(template).Error; err != nil {
		t.Fatal(err)
	}
	return template
}

func createProviderPath(t *testing.T, db *gorm.DB, channelID uint, messageType, providerCode string) (*model.ProviderAccount, *model.ProviderTemplate, *model.ChannelTemplateBinding) {
	t.Helper()
	account := &model.ProviderAccount{
		AccountCode:  fmt.Sprintf("account-%s-%d", providerCode, timeNowUnixNano()),
		AccountName:  providerCode,
		ProviderCode: providerCode,
		ProviderType: messageType,
		Config:       `{}`,
		Status:       1,
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatal(err)
	}
	providerTemplate := &model.ProviderTemplate{
		ProviderID:   account.ID,
		TemplateCode: fmt.Sprintf("tpl-%d", account.ID),
		TemplateName: "provider",
		Status:       1,
	}
	if err := providerTemplate.SetVariables([]string{"code"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(providerTemplate).Error; err != nil {
		t.Fatal(err)
	}
	binding := createBinding(t, db, channelID, providerTemplate.ID, account.ID, 1, 1, "")
	return account, providerTemplate, binding
}

func createBinding(t *testing.T, db *gorm.DB, channelID, providerTemplateID, accountID uint, status, active int8, mapping string) *model.ChannelTemplateBinding {
	t.Helper()
	binding := &model.ChannelTemplateBinding{
		ChannelID:          channelID,
		ProviderTemplateID: providerTemplateID,
		ProviderID:         accountID,
		ParamMapping:       mapping,
		Weight:             10,
		Priority:           100,
		Status:             status,
		IsActive:           active,
	}
	if err := db.Create(binding).Error; err != nil {
		t.Fatal(err)
	}
	return binding
}

func createSignatureAlias(t *testing.T, db *gorm.DB, channelID, accountID uint, alias string) *model.ChannelSignatureMapping {
	t.Helper()
	signature := &model.ProviderSignature{
		ProviderAccountID: accountID,
		SignatureCode:     "signature-" + alias,
		SignatureName:     alias,
		Status:            1,
	}
	if err := db.Create(signature).Error; err != nil {
		t.Fatal(err)
	}
	mapping := &model.ChannelSignatureMapping{
		ChannelID:           channelID,
		SignatureName:       alias,
		ProviderSignatureID: signature.ID,
		ProviderID:          accountID,
		Status:              1,
	}
	if err := db.Create(mapping).Error; err != nil {
		t.Fatal(err)
	}
	return mapping
}

func validationHasCode(err error, code string) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr) && containsString(validationErr.Codes, code)
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func stringsContains(value, substring string) bool {
	for i := 0; i+len(substring) <= len(value); i++ {
		if value[i:i+len(substring)] == substring {
			return true
		}
	}
	return false
}

var testSequence int64

func timeNowUnixNano() int64 {
	testSequence++
	return testSequence
}
