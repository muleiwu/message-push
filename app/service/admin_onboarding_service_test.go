package service

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/readiness"
	registry "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const (
	testOnboardingRequiredProvider = "onboarding_required_sms"
	testOnboardingOptionalProvider = "onboarding_optional_email"
)

func TestAdminOnboardingSummaryWorkflowAndPerChannelVerification(t *testing.T) {
	registerOnboardingProviders(t)
	db := newOnboardingTestDB(t)
	service := &AdminOnboardingService{
		db:                 db,
		readinessEvaluator: readiness.NewChannelEvaluator(db),
	}

	empty, err := service.GetSummary()
	if err != nil {
		t.Fatal(err)
	}
	if empty.ReadyForTest || empty.ReadyForAPISend || empty.Steps.MessageTemplates.Total != 0 ||
		empty.Steps.ProviderSignatures.Status != constants.OnboardingStepNotApplicable {
		t.Fatalf("unexpected empty summary: %+v", empty)
	}
	for _, code := range []string{
		constants.OnboardingBlockerMessageTemplateNotConfigured,
		constants.OnboardingBlockerProviderAccountNotConfigured,
		constants.OnboardingBlockerProviderTemplateNotConfigured,
		constants.OnboardingBlockerChannelNotReady,
		constants.OnboardingBlockerApplicationNotConfigured,
		constants.OnboardingBlockerAdminTestNotCompleted,
	} {
		if !onboardingHasBlocker(empty, code) {
			t.Fatalf("empty summary blockers do not contain %s: %+v", code, empty.PriorityBlockers)
		}
	}

	smsChannel, smsAccount := createOnboardingChannel(t, db, constants.MessageTypeSMS, testOnboardingRequiredProvider)
	partial, err := service.GetSummary()
	if err != nil {
		t.Fatal(err)
	}
	if partial.Steps.MessageTemplates.Status != constants.OnboardingStepComplete ||
		partial.Steps.ProviderAccounts.Status != constants.OnboardingStepComplete ||
		partial.Steps.ProviderTemplates.Status != constants.OnboardingStepComplete ||
		partial.Steps.ProviderSignatures.RequiredAccountCount != 1 ||
		partial.Steps.ProviderSignatures.ConfiguredAccountCount != 0 ||
		partial.Steps.Channels.Blocked != 1 || partial.ReadyForTest {
		t.Fatalf("unexpected partially configured summary: %+v", partial)
	}
	if !onboardingHasBlocker(partial, constants.OnboardingBlockerProviderSignatureNotConfigured) {
		t.Fatalf("missing required signature must be prioritized: %+v", partial.PriorityBlockers)
	}

	smsSignature := &model.ProviderSignature{
		ProviderAccountID: smsAccount.ID,
		SignatureCode:     "approved-sms-signature",
		SignatureName:     "approved-sms-signature",
		Status:            1,
	}
	if err := db.Create(smsSignature).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ChannelSignatureMapping{
		ChannelID:           smsChannel.ID,
		SignatureName:       "default",
		ProviderSignatureID: smsSignature.ID,
		ProviderID:          smsAccount.ID,
		Status:              1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	testable, err := service.GetSummary()
	if err != nil {
		t.Fatal(err)
	}
	if !testable.ReadyForTest || testable.ReadyForAPISend || testable.Steps.Channels.Ready != 1 {
		t.Fatalf("expected a testable but not API-ready branch: %+v", testable)
	}

	emailChannel, emailAccount := createOnboardingChannel(t, db, constants.MessageTypeEmail, testOnboardingOptionalProvider)
	// A disabled legacy email subject is visible in history, but does not count
	// toward the required-signature setup step or its abnormal total.
	if err := db.Create(&model.ProviderSignature{
		ProviderAccountID: emailAccount.ID,
		SignatureCode:     "legacy-subject",
		SignatureName:     "legacy-subject",
		Status:            0,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Application{
		AppID:     "onboarding-app",
		AppSecret: "secret",
		AppName:   "Onboarding App",
		Status:    1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	testTime := time.Now().UTC().Add(time.Second)
	createOnboardingTask(t, db, "successful-channel-test", smsChannel.ID, constants.TaskStatusSuccess, testTime)
	createOnboardingTask(t, db, "later-unrelated-failure", emailChannel.ID, constants.TaskStatusFailed, testTime.Add(2*time.Second))

	complete, err := service.GetSummary()
	if err != nil {
		t.Fatal(err)
	}
	if !complete.ReadyForTest || !complete.ReadyForAPISend {
		t.Fatalf("a later unrelated failure must not invalidate a verified channel: %+v", complete)
	}
	if complete.LatestAdminTest == nil || complete.LatestAdminTest.TaskID != "later-unrelated-failure" ||
		complete.LatestAdminTest.Status != constants.TaskStatusFailed {
		t.Fatalf("global latest test should remain factual and independent: %+v", complete.LatestAdminTest)
	}
	if complete.Steps.ProviderSignatures.Total != 1 || complete.Steps.ProviderSignatures.Enabled != 1 ||
		complete.Steps.ProviderSignatures.Abnormal != 0 || complete.Steps.ProviderSignatures.RequiredAccountCount != 1 ||
		complete.Steps.ProviderSignatures.ConfiguredAccountCount != 1 || complete.Steps.ProviderSignatures.NotApplicableAccountCount != 1 {
		t.Fatalf("legacy email signature polluted required signature progress: %+v", complete.Steps.ProviderSignatures)
	}
	if onboardingHasBlocker(complete, constants.OnboardingBlockerAdminTestNotCompleted) ||
		onboardingHasBlocker(complete, constants.OnboardingBlockerConfigurationChanged) {
		t.Fatalf("verified channel should remove test blockers: %+v", complete.PriorityBlockers)
	}

	smsLatest, err := service.latestAdminTestForChannel(smsChannel.ID)
	if err != nil {
		t.Fatal(err)
	}
	if smsLatest == nil || smsLatest.TaskID != "successful-channel-test" || smsLatest.ConfigChanged {
		t.Fatalf("unexpected channel-specific latest test: %+v", smsLatest)
	}

	createOnboardingTask(t, db, "later-same-channel-failure", smsChannel.ID, constants.TaskStatusFailed, testTime.Add(3*time.Second))
	failedAgain, err := service.GetSummary()
	if err != nil {
		t.Fatal(err)
	}
	if failedAgain.ReadyForAPISend || !onboardingHasBlocker(failedAgain, constants.OnboardingBlockerAdminTestNotCompleted) {
		t.Fatalf("latest failure on every sendable channel must require another test: %+v", failedAgain)
	}

	equalSecond := testTime.Add(4 * time.Second).Truncate(time.Second)
	createOnboardingTask(t, db, "same-second-success", smsChannel.ID, constants.TaskStatusSuccess, equalSecond)
	if err := db.Model(&model.Channel{}).Where("id = ?", smsChannel.ID).UpdateColumn("updated_at", equalSecond).Error; err != nil {
		t.Fatal(err)
	}
	sameSecondChanged, err := service.GetSummary()
	if err != nil {
		t.Fatal(err)
	}
	if sameSecondChanged.ReadyForAPISend || !onboardingHasBlocker(sameSecondChanged, constants.OnboardingBlockerConfigurationChanged) {
		t.Fatalf("same-second configuration change must fail closed: %+v", sameSecondChanged)
	}
}

func TestAdminOnboardingRejectsProviderTemplateWithInvalidVariables(t *testing.T) {
	registerOnboardingProviders(t)
	db := newOnboardingTestDB(t)
	service := &AdminOnboardingService{db: db, readinessEvaluator: readiness.NewChannelEvaluator(db)}
	account := &model.ProviderAccount{
		AccountCode:  "invalid-template-account",
		AccountName:  "Invalid template account",
		ProviderCode: testOnboardingOptionalProvider,
		ProviderType: constants.MessageTypeEmail,
		Config:       `{}`,
		Status:       1,
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ProviderTemplate{
		ProviderID:   account.ID,
		TemplateCode: "invalid-variables",
		TemplateName: "Invalid variables",
		Variables:    `{not-json}`,
		Status:       1,
	}).Error; err != nil {
		t.Fatal(err)
	}

	summary, err := service.GetSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Steps.ProviderTemplates.Status != constants.OnboardingStepIncomplete ||
		!onboardingHasBlocker(summary, constants.OnboardingBlockerProviderTemplateNotConfigured) {
		t.Fatalf("invalid variables must not count as a usable provider template: %+v", summary)
	}
}

func newOnboardingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "onboarding.db")), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
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
		`CREATE TABLE applications (id INTEGER PRIMARY KEY AUTOINCREMENT, app_id TEXT NOT NULL UNIQUE, app_secret TEXT NOT NULL, app_name TEXT NOT NULL, status INTEGER, ip_whitelist TEXT, webhook_url TEXT, daily_quota INTEGER, rate_limit INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE push_tasks (id INTEGER PRIMARY KEY AUTOINCREMENT, task_id TEXT NOT NULL UNIQUE, app_id TEXT NOT NULL, channel_id INTEGER NOT NULL, message_type TEXT NOT NULL, receiver TEXT NOT NULL, template_code TEXT, template_params TEXT, signature TEXT, status TEXT NOT NULL, callback_status TEXT, callback_time DATETIME, retry_count INTEGER, max_retry INTEGER, exclude_provider_ids TEXT, scheduled_at DATETIME, created_at DATETIME, updated_at DATETIME)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create onboarding schema: %v", err)
		}
	}
	return db
}

func registerOnboardingProviders(t *testing.T) {
	t.Helper()
	providers := []*registry.ProviderMeta{
		{Code: testOnboardingRequiredProvider, Name: "required sms", Type: constants.MessageTypeSMS, RequiresSignature: true},
		{Code: testOnboardingOptionalProvider, Name: "optional email", Type: constants.MessageTypeEmail},
	}
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil && !strings.Contains(err.Error(), "already registered") {
			t.Fatalf("register provider %s: %v", provider.Code, err)
		}
	}
}

func createOnboardingChannel(t *testing.T, db *gorm.DB, messageType, providerCode string) (*model.Channel, *model.ProviderAccount) {
	t.Helper()
	template := &model.MessageTemplate{
		TemplateName: fmt.Sprintf("%s system template", messageType),
		Content:      "code={code}",
		Status:       1,
	}
	if err := template.SetVariables([]string{"code"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(template).Error; err != nil {
		t.Fatal(err)
	}
	channel := &model.Channel{
		Name:              fmt.Sprintf("%s channel", messageType),
		Type:              messageType,
		MessageTemplateID: template.ID,
		Status:            1,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	account := &model.ProviderAccount{
		AccountCode:  fmt.Sprintf("%s-account", messageType),
		AccountName:  fmt.Sprintf("%s account", messageType),
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
		TemplateCode: fmt.Sprintf("%s-template", messageType),
		TemplateName: fmt.Sprintf("%s provider template", messageType),
		Status:       1,
	}
	if err := providerTemplate.SetVariables([]string{"code"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(providerTemplate).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ChannelTemplateBinding{
		ChannelID:          channel.ID,
		ProviderTemplateID: providerTemplate.ID,
		ProviderID:         account.ID,
		Weight:             10,
		Priority:           100,
		Status:             1,
		IsActive:           1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return channel, account
}

func createOnboardingTask(t *testing.T, db *gorm.DB, taskID string, channelID uint, status string, createdAt time.Time) {
	t.Helper()
	if err := db.Create(&model.PushTask{
		TaskID:      taskID,
		AppID:       adminTestAppID,
		ChannelID:   channelID,
		MessageType: constants.MessageTypeSMS,
		Receiver:    "13800138000",
		Status:      status,
		CreatedAt:   createdAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func onboardingHasBlocker(summary *dto.OnboardingSummaryResponse, code string) bool {
	for _, blocker := range summary.PriorityBlockers {
		if blocker != nil && blocker.Code == code {
			return true
		}
	}
	return false
}
