package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/readiness"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	"gorm.io/gorm"
)

type resourceServiceFixture struct {
	s            *AdminProviderResourceService
	account      *model.ProviderAccount
	rows         map[domain.ResourceKind]map[string]domain.RemoteResource
	writes       int
	lastInput    domain.ResourceInput
	queryFailure bool
}

func newResourceServiceFixture(t *testing.T) *resourceServiceFixture {
	t.Helper()
	db := newOnboardingTestDB(t)
	f := &resourceServiceFixture{s: NewAdminProviderResourceServiceWithDB(db), rows: map[domain.ResourceKind]map[string]domain.RemoteResource{domain.ResourceTemplates: {}, domain.ResourceSignatures: {}}}
	f.account = &model.ProviderAccount{AccountCode: "resource-account", AccountName: "掌榕", ProviderCode: constants.ProviderZrwinfoSMS, ProviderType: "sms", Config: "{}", Status: 1}
	if err := db.Create(f.account).Error; err != nil {
		t.Fatal(err)
	}
	registered, err := domain.GetByCode(constants.ProviderZrwinfoSMS)
	if err != nil {
		t.Fatal(err)
	}
	meta := *registered
	meta.Resources = map[domain.ResourceKind]*domain.ResourceDefinition{}
	for kind, definition := range registered.Resources {
		copyDef := *definition
		copyDef.Operations = map[domain.ResourceAction]*domain.ResourceOperation{}
		for action, op := range definition.Operations {
			copyOp := *op
			copyOp.Handler = func(_ context.Context, _ *model.ProviderAccount, input domain.ResourceInput) ([]domain.RemoteResource, error) {
				if action == domain.ResourceQuery {
					if f.queryFailure {
						return nil, fmt.Errorf("query failed")
					}
					result := []domain.RemoteResource{}
					for _, row := range f.rows[kind] {
						if input.ID == "" || input.ID == row.ID {
							result = append(result, row)
						}
					}
					if input.ID != "" && len(result) == 0 {
						return nil, fmt.Errorf("not found")
					}
					return result, nil
				}
				f.writes++
				f.lastInput = input
				id := input.ID
				if id == "" {
					id = "900"
				}
				input.ID = id
				if action == domain.ResourceDelete {
					delete(f.rows[kind], id)
				} else {
					f.rows[kind][id] = domain.RemoteResource{ResourceInput: input, AuditStatus: 1}
				}
				return []domain.RemoteResource{{ResourceInput: domain.ResourceInput{ID: id}}}, nil
			}
			copyDef.Operations[action] = &copyOp
		}
		meta.Resources[kind] = &copyDef
	}
	f.s.lookup = func(string) (*domain.ProviderMeta, error) { return &meta, nil }
	return f
}

func resourceSelection(item *ResourcePreviewItem, action string) ResourceSelection {
	return ResourceSelection{ID: item.Remote.ID, Version: item.Version, Action: action}
}

func TestResourceLiteralDollarTemplateCanBeImported(t *testing.T) {
	f := newResourceServiceFixture(t)
	ctx := context.Background()
	kind := domain.ResourceTemplates
	const original = "主机${host_name}的监控规则“{rule_name}”触发时延告警，当前时延{value}毫秒，告警阈值{threshold}毫秒，请及时排查。"
	f.rows[kind]["11"] = domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: "11", Content: original}, AuditStatus: 2}
	rows, err := f.s.Preview(ctx, f.account.ID, kind, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("preview: %+v %v", rows, err)
	}
	if rows[0].Error != "" || rows[0].Parsed == nil {
		t.Fatalf("valid original was rejected: %+v", rows[0])
	}
	if rows[0].Remote.Content != original {
		t.Fatal("preview changed original text")
	}
	_, err = f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(rows[0], "create")}})
	if err != nil {
		t.Fatal(err)
	}
	var record model.ProviderTemplate
	if err = f.s.db.Preload("ProviderAccount").Where("provider_id = ? AND template_code = ?", f.account.ID, "11").First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.TemplateContent != original || !record.Usable() || !readiness.ProviderTemplateVariablesValid(&record) {
		t.Fatalf("imported resource is not usable: %+v", record)
	}
}

func TestResourceNamedTemplatePreviewImportReadinessAndSendOrder(t *testing.T) {
	f := newResourceServiceFixture(t)
	ctx := context.Background()
	kind := domain.ResourceTemplates
	original := "主机{host_name}的规则“{rule_name}”已恢复，再检查{host_name}。"
	f.rows[kind]["3344800"] = domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: "3344800", Content: original}, AuditStatus: 2}
	rows, err := f.s.Preview(ctx, f.account.ID, kind, "")
	if err != nil || len(rows) != 1 || rows[0].Error != "" {
		t.Fatalf("named preview rejected: %+v %v", rows, err)
	}
	if rows[0].Remote.Content != original || !reflect.DeepEqual(rows[0].Parsed.NativeVariables, []string{"{host_name}", "{rule_name}"}) {
		t.Fatalf("original changed: %+v", rows[0])
	}
	_, err = f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(rows[0], "create")}})
	if err != nil {
		t.Fatal(err)
	}
	var record model.ProviderTemplate
	if err = f.s.db.Preload("ProviderAccount").Where("provider_id = ? AND template_code = ?", f.account.ID, "3344800").First(&record).Error; err != nil {
		t.Fatal(err)
	}
	if record.TemplateContent != original || !record.Usable() || !readiness.ProviderTemplateVariablesValid(&record) {
		t.Fatalf("imported named resource not usable: %+v", record)
	}
	bindings := &model.ChannelTemplateBinding{ProviderID: f.account.ID, ProviderTemplateID: record.ID, ProviderTemplate: &record, MappedContentVersion: record.ContentVersion, Status: 1, IsActive: 1, Weight: 1}
	if err = bindings.SetParamMapping([]model.ParamMappingItem{{Type: model.ParamMappingTypeMapping, ProviderVar: "host_name", SystemVar: "host"}, {Type: model.ParamMappingTypeMapping, ProviderVar: "rule_name", SystemVar: "rule"}}); err != nil {
		t.Fatal(err)
	}
	if issues := readiness.ValidateBinding("sms", []string{"host", "rule"}, bindings); len(issues) > 0 {
		t.Fatalf("binding rejected named variables: %v", issues)
	}
	parsed, err := domain.ParseProviderTemplate(&record)
	if err != nil {
		t.Fatal(err)
	}
	values, err := parsed.Bind(map[string]string{"host_name": "server01", "rule_name": "health"})
	if err != nil || !reflect.DeepEqual(values.Ordered, []string{"server01", "health", "server01"}) {
		t.Fatalf("wrong send order: %+v %v", values, err)
	}
	if err := f.s.db.Omit("ProviderTemplate").Create(bindings).Error; err != nil {
		t.Fatal(err)
	}
	updated := f.rows[kind]["3344800"]
	updated.Content = "规则{rule_name}，主机{host_name}。"
	f.rows[kind]["3344800"] = updated
	rows, err = f.s.Preview(ctx, f.account.ID, kind, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(rows[0], "update")}})
	if err != nil {
		t.Fatal(err)
	}
	if err = f.s.db.Preload("ProviderAccount").First(&record, record.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.db.First(bindings, bindings.ID).Error; err != nil {
		t.Fatal(err)
	}
	bindings.ProviderTemplate = &record
	if issues := readiness.ValidateBinding("sms", []string{"host", "rule"}, bindings); len(issues) == 0 {
		t.Fatal("changed body retained eligible binding")
	}
	after, _ := bindings.GetParamMapping()
	if len(after) != 0 || bindings.MappedContentVersion != 0 || record.ContentVersion != 2 {
		t.Fatalf("mapping not reset: %+v %+v", bindings, record)
	}
}

func TestResourcePreviewImportConflictAndLocalPolicy(t *testing.T) {
	f := newResourceServiceFixture(t)
	ctx := context.Background()
	kind := domain.ResourceTemplates
	f.rows[kind]["11"] = domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: "11", Content: "验证码{1}", Category: "1"}, AuditStatus: 1}
	preview, err := f.s.Preview(ctx, f.account.ID, kind, "")
	if err != nil || len(preview) != 1 {
		t.Fatalf("preview %v %v", preview, err)
	}
	var count int64
	f.s.db.Model(&model.ProviderTemplate{}).Count(&count)
	if count != 0 {
		t.Fatal("preview wrote data")
	}
	result, err := f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(preview[0], "create")}})
	if err != nil || result.Created != 1 {
		t.Fatalf("import %v %v", result, err)
	}
	var local model.ProviderTemplate
	if err = f.s.db.First(&local).Error; err != nil {
		t.Fatal(err)
	}
	if local.TemplateCode != "11" || local.TemplateContent != "验证码{1}" || local.Usable() {
		t.Fatalf("wrong mirror: %+v", local)
	}
	id := local.ID
	f.s.db.Model(&local).Updates(map[string]any{"template_name": "本地名称", "remark": "本地备注", "status": 0})
	old, _ := f.s.Preview(ctx, f.account.ID, kind, "")
	r := f.rows[kind]["11"]
	r.AuditStatus = 2
	r.Content = "验证码{1}，有效期{2}"
	f.rows[kind]["11"] = r
	_, err = f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(old[0], "update")}})
	if !errors.Is(err, ErrResourceConflict) {
		t.Fatalf("stale version accepted: %v", err)
	}
	fresh, _ := f.s.Preview(ctx, f.account.ID, kind, "")
	_, err = f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(fresh[0], "update")}})
	if err != nil {
		t.Fatal(err)
	}
	f.s.db.First(&local, id)
	if local.ID != id || local.TemplateName != "本地名称" || local.Remark != "本地备注" || local.Status != 0 || *local.AuditStatus != 2 || local.TemplateContent != "验证码{1}，有效期{2}" {
		t.Fatalf("local preferences changed: %+v", local)
	}
	f.s.db.Delete(&local)
	deleted, _ := f.s.Preview(ctx, f.account.ID, kind, "")
	if !deleted[0].Deleted {
		t.Fatal("deleted record not recognized")
	}
	_, err = f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(deleted[0], "restore")}})
	if err != nil {
		t.Fatal(err)
	}
	f.s.db.Model(&model.ProviderTemplate{}).Count(&count)
	if count != 1 {
		t.Fatal("restore duplicated record")
	}
}

func TestResourceImportSignatureIdentityAndAccountIsolation(t *testing.T) {
	f := newResourceServiceFixture(t)
	kind := domain.ResourceSignatures
	ctx := context.Background()
	local := &model.ProviderSignature{ProviderAccountID: f.account.ID, SignatureCode: "品牌", SignatureName: "本地名称", Remark: "备注", Status: 1}
	if err := f.s.db.Create(local).Error; err != nil {
		t.Fatal(err)
	}
	f.rows[kind]["77"] = domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: "77", Content: "【品牌】"}, AuditStatus: 2}
	preview, err := f.s.Preview(ctx, f.account.ID, kind, "")
	if err != nil || preview[0].LocalID != local.ID {
		t.Fatalf("legacy signature match: %v %v", preview, err)
	}
	_, err = f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(preview[0], "skip")}})
	if err != nil {
		t.Fatal(err)
	}
	f.s.db.First(local)
	if local.RemoteID != "" {
		t.Fatal("skip modified record")
	}
	_, err = f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(preview[0], "update")}})
	if err != nil {
		t.Fatal(err)
	}
	f.s.db.First(local)
	if local.RemoteID != "77" || local.SignatureCode != "【品牌】" || local.SignatureName != "本地名称" {
		t.Fatalf("wrong signature mirror: %+v", local)
	}
	other := *f.account
	other.ID = 0
	other.AccountCode = "other"
	f.s.db.Create(&other)
	preview, err = f.s.Preview(ctx, other.ID, kind, "")
	if err != nil || preview[0].LocalID != 0 {
		t.Fatalf("account boundary: %+v %v", preview, err)
	}
}

func TestResourceImportRollbackAndRepeatedSubmission(t *testing.T) {
	f := newResourceServiceFixture(t)
	ctx := context.Background()
	kind := domain.ResourceSignatures
	for i := 1; i <= 2; i++ {
		id := fmt.Sprint(i)
		f.rows[kind][id] = domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: id, Content: "品牌" + id}, AuditStatus: 2}
	}
	preview, _ := f.s.Preview(ctx, f.account.ID, kind, "")
	selections := []ResourceSelection{resourceSelection(preview[0], "create"), resourceSelection(preview[1], "create")}
	selections[1].Version = "outdated"
	if _, err := f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: selections}); err == nil {
		t.Fatal("invalid batch accepted")
	}
	var count int64
	f.s.db.Model(&model.ProviderSignature{}).Count(&count)
	if count != 0 {
		t.Fatal("batch partially committed")
	}
	req := ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(preview[0], "create")}}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := f.s.Import(ctx, f.account.ID, req); results <- err }()
	}
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		if err == nil {
			ok++
		} else if !errors.Is(err, ErrResourceConflict) {
			t.Fatal(err)
		}
	}
	f.s.db.Model(&model.ProviderSignature{}).Count(&count)
	if ok != 1 || count != 1 {
		t.Fatalf("duplicate submission: successes=%d rows=%d", ok, count)
	}
}

func TestResourceMutationMirrorAndRemoteSuccessLocalFailure(t *testing.T) {
	f := newResourceServiceFixture(t)
	ctx := context.Background()
	kind := domain.ResourceTemplates
	req := ResourceMutationRequest{ResourceInput: domain.ResourceInput{Name: "模板", Content: "验证码{code}", Category: "1", Description: "登录"}}
	result, err := f.s.Mutate(ctx, f.account.ID, kind, domain.ResourceCreate, req)
	if err != nil || result.LocalID == 0 || f.lastInput.Content != "验证码{code}" {
		t.Fatalf("create: %+v %v, request=%+v", result, err, f.lastInput)
	}
	var local model.ProviderTemplate
	f.s.db.First(&local, result.LocalID)
	if local.TemplateContent != "验证码{code}" || local.Usable() {
		t.Fatalf("wrong compiled mirror %+v", local)
	}
	preview, _ := f.s.Preview(ctx, f.account.ID, kind, "900")
	linkedChannel := &model.Channel{Name: "资源引用通道", Type: "sms", Status: 1}
	if err := f.s.db.Create(linkedChannel).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.db.Create(&model.ChannelTemplateBinding{ChannelID: linkedChannel.ID, ProviderID: f.account.ID, ProviderTemplateID: result.LocalID, Status: 1, IsActive: 1, Weight: 1}).Error; err != nil {
		t.Fatal(err)
	}
	preview, _ = f.s.Preview(ctx, f.account.ID, kind, "900")
	_, err = f.s.Mutate(ctx, f.account.ID, kind, domain.ResourceDelete, ResourceMutationRequest{ResourceInput: domain.ResourceInput{ID: "900"}, Version: preview[0].Version})
	if err == nil || f.writes != 1 {
		t.Fatal("delete did not require impact confirmation")
	}
	invalidated := []uint{}
	f.s.invalidate = func(id uint) { invalidated = append(invalidated, id) }
	deleted, err := f.s.Mutate(ctx, f.account.ID, kind, domain.ResourceDelete, ResourceMutationRequest{ResourceInput: domain.ResourceInput{ID: "900"}, Version: preview[0].Version, ConfirmImpact: true})
	if err != nil || deleted.PendingSync {
		t.Fatalf("delete: %+v %v", deleted, err)
	}
	f.s.db.First(&local, result.LocalID)
	if !local.RemoteDeleted || local.Usable() {
		t.Fatal("deleted resource still usable")
	}
	if len(invalidated) != 2 || invalidated[0] != linkedChannel.ID || invalidated[1] != linkedChannel.ID {
		t.Fatalf("cache invalidation: %v", invalidated)
	}
	// A provider write cannot be rolled back by a failing local transaction.
	if err := f.s.db.Callback().Create().Before("gorm:create").Register("resource_test_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "provider_signatures" {
			tx.AddError(errors.New("local database failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	partial, err := f.s.Mutate(ctx, f.account.ID, domain.ResourceSignatures, domain.ResourceCreate, ResourceMutationRequest{ResourceInput: domain.ResourceInput{Content: "品牌"}})
	if err != nil || !partial.PendingSync || partial.RemoteID != "900" || f.writes != 3 {
		t.Fatalf("partial outcome: %+v %v writes=%d", partial, err, f.writes)
	}
	if err := f.s.db.Callback().Create().Remove("resource_test_failure"); err != nil {
		t.Fatal(err)
	}
	rows, _ := f.s.Preview(ctx, f.account.ID, domain.ResourceSignatures, "")
	_, err = f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: domain.ResourceSignatures, Selections: []ResourceSelection{resourceSelection(rows[0], "create")}})
	if err != nil {
		t.Fatal(err)
	}
	if f.writes != 3 {
		t.Fatal("recovery repeated remote mutation")
	}
}

func TestResourceMutationsCanBeRegisteredWithoutQuery(t *testing.T) {
	f := newResourceServiceFixture(t)
	ctx := context.Background()
	kind := domain.ResourceSignatures
	meta, _ := f.s.lookup(f.account.ProviderCode)
	delete(meta.Resources[kind].Operations, domain.ResourceQuery)
	created, err := f.s.Mutate(ctx, f.account.ID, kind, domain.ResourceCreate, ResourceMutationRequest{ResourceInput: domain.ResourceInput{Content: "原签名"}})
	if err != nil || created.LocalID == 0 {
		t.Fatalf("create without query: %+v %v", created, err)
	}
	items, err := f.s.Workspace(ctx, f.account.ID, kind)
	if err != nil || len(items) != 1 {
		t.Fatalf("local operation workspace: %+v %v", items, err)
	}
	updated, err := f.s.Mutate(ctx, f.account.ID, kind, domain.ResourceUpdate, ResourceMutationRequest{ResourceInput: domain.ResourceInput{ID: "900", Content: "新签名"}, Version: items[0].Version})
	if err != nil || updated.PendingSync || f.writes != 2 {
		t.Fatalf("update without query: %+v %v", updated, err)
	}
	if _, err = f.s.Preview(ctx, f.account.ID, kind, ""); !errors.Is(err, domain.ErrResourceUnsupported) {
		t.Fatalf("undeclared query: %v", err)
	}
}

func TestResourceMatchingPrefersLiveCopiesAndBlocksAmbiguousDeletion(t *testing.T) {
	f := newResourceServiceFixture(t)
	ctx := context.Background()
	kind := domain.ResourceTemplates
	f.rows[kind]["11"] = domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: "11", Content: "验证码{1}"}, AuditStatus: 2}
	old := &model.ProviderTemplate{ProviderID: f.account.ID, TemplateCode: "11", TemplateName: "历史", Status: 1}
	if err := f.s.db.Create(old).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.s.db.Delete(old).Error; err != nil {
		t.Fatal(err)
	}
	current := &model.ProviderTemplate{ProviderID: f.account.ID, TemplateCode: "11", TemplateName: "当前", Status: 1}
	if err := f.s.db.Create(current).Error; err != nil {
		t.Fatal(err)
	}
	items, err := f.s.Preview(ctx, f.account.ID, kind, "")
	if err != nil || items[0].LocalID != current.ID || items[0].IdentityConflict {
		t.Fatalf("current match: %+v %v", items, err)
	}
	duplicate := &model.ProviderTemplate{ProviderID: f.account.ID, TemplateCode: "11", TemplateName: "重复", Status: 1}
	if err := f.s.db.Create(duplicate).Error; err != nil {
		t.Fatal(err)
	}
	items, err = f.s.Preview(ctx, f.account.ID, kind, "")
	if err != nil || !items[0].IdentityConflict {
		t.Fatalf("ambiguity: %+v %v", items, err)
	}
	_, err = f.s.Mutate(ctx, f.account.ID, kind, domain.ResourceDelete, ResourceMutationRequest{ResourceInput: domain.ResourceInput{ID: "11"}, Version: items[0].Version, ConfirmImpact: true})
	if err == nil || f.writes != 0 {
		t.Fatal("ambiguous deletion executed")
	}
}

func TestNativeTemplateManualIdentityAndExactBodyReset(t *testing.T) {
	f := newResourceServiceFixture(t)
	ctx := context.Background()
	local := &model.ProviderTemplate{ProviderID: f.account.ID, TemplateCode: "manual-code", TemplateName: "本地名称", TemplateContent: "主机{host}", Status: 1, ContentVersion: 1}
	if err := f.s.db.Create(local).Error; err != nil {
		t.Fatal(err)
	}
	binding := &model.ChannelTemplateBinding{ChannelID: 1, ProviderID: f.account.ID, ProviderTemplateID: local.ID, ParamMapping: `[{"type":"fixed","provider_var":"host","value":"server"}]`, MappedContentVersion: 1, Status: 1, IsActive: 1, Weight: 1}
	if err := f.s.db.Create(binding).Error; err != nil {
		t.Fatal(err)
	}
	kind := domain.ResourceTemplates
	f.rows[kind][local.TemplateCode] = domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: local.TemplateCode, Content: local.TemplateContent}, AuditStatus: 2}
	importAction := func(action string) {
		t.Helper()
		rows, err := f.s.Preview(ctx, f.account.ID, kind, local.TemplateCode)
		if err != nil || len(rows) != 1 || rows[0].LocalID != local.ID {
			t.Fatalf("identity match: %+v %v", rows, err)
		}
		if _, err = f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(rows[0], action)}}); err != nil {
			t.Fatal(err)
		}
	}
	importAction("update")
	var saved model.ProviderTemplate
	f.s.db.First(&saved, local.ID)
	f.s.db.First(binding, binding.ID)
	if saved.ContentVersion != 1 || binding.MappedContentVersion != 1 || binding.ParamMapping == "[]" {
		t.Fatal("same body reset mappings")
	}
	remote := f.rows[kind][local.TemplateCode]
	remote.Content += "。"
	f.rows[kind][local.TemplateCode] = remote
	importAction("skip")
	f.s.db.First(&saved, local.ID)
	f.s.db.First(binding, binding.ID)
	if saved.ContentVersion != 1 || binding.MappedContentVersion != 1 {
		t.Fatal("preview/skip mutated state")
	}
	importAction("update")
	f.s.db.First(&saved, local.ID)
	f.s.db.First(binding, binding.ID)
	if saved.ID != local.ID || saved.TemplateName != "本地名称" || saved.ContentVersion != 2 || saved.TemplateContent != remote.Content || binding.MappedContentVersion != 0 || binding.ParamMapping != "[]" {
		t.Fatalf("body change did not reset: %+v %+v", saved, binding)
	}
	importAction("update")
	f.s.db.First(&saved, local.ID)
	if saved.ContentVersion != 2 {
		t.Fatal("repeated import incremented version")
	}
}

func TestResourceUncertainUpdateStopsExistingMirror(t *testing.T) {
	f := newResourceServiceFixture(t)
	ctx := context.Background()
	kind := domain.ResourceTemplates
	f.rows[kind]["11"] = domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: "11", Name: "测试", Content: "主机{host}"}, AuditStatus: 2}
	rows, _ := f.s.Preview(ctx, f.account.ID, kind, "")
	if _, err := f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(rows[0], "create")}}); err != nil {
		t.Fatal(err)
	}
	var local model.ProviderTemplate
	f.s.db.First(&local)
	binding := model.ChannelTemplateBinding{ChannelID: 1, ProviderID: f.account.ID, ProviderTemplateID: local.ID, Status: 1, IsActive: 1, Weight: 1, MappedContentVersion: 1, ParamMapping: `[{"type":"fixed","provider_var":"host","value":"server"}]`}
	if err := f.s.db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	meta, _ := f.s.lookup(f.account.ProviderCode)
	calls := 0
	meta.Resources[kind].Operations[domain.ResourceUpdate].Handler = func(context.Context, *model.ProviderAccount, domain.ResourceInput) ([]domain.RemoteResource, error) {
		calls++
		return nil, &domain.RemoteResourceError{Code: "TIMEOUT", Message: "结果不确定", Uncertain: true}
	}
	rows, _ = f.s.Preview(ctx, f.account.ID, kind, "11")
	_, err := f.s.Mutate(ctx, f.account.ID, kind, domain.ResourceUpdate, ResourceMutationRequest{ResourceInput: domain.ResourceInput{ID: "11", Name: "测试", Content: "新正文{host}", Description: "通知"}, Version: rows[0].Version, ConfirmImpact: true})
	if err == nil || calls != 1 {
		t.Fatalf("uncertain request repeated: %v %d", err, calls)
	}
	f.s.db.First(&local, local.ID)
	f.s.db.First(&binding, binding.ID)
	if local.Usable() || binding.MappedContentVersion != 0 || binding.ParamMapping != "[]" {
		t.Fatal("old configuration remained usable after uncertain write")
	}
}

func TestTemplateParsingDoesNotRequireRemoteCRUD(t *testing.T) {
	f := newResourceServiceFixture(t)
	account := &model.ProviderAccount{AccountCode: "aliyun-local", AccountName: "本地", ProviderCode: constants.ProviderAliyunSMS, ProviderType: "sms", Config: "{}", Status: 1}
	if err := f.s.db.Create(account).Error; err != nil {
		t.Fatal(err)
	}
	f.s.lookup = domain.GetByCode
	parsed, err := f.s.Parse(context.Background(), account.ID, "验证码${code}")
	if err != nil || parsed.SystemContent != "验证码{code}" || len(parsed.Variables) != 1 || parsed.Variables[0] != "code" {
		t.Fatalf("parse-only provider: %+v %v", parsed, err)
	}
	if f.writes != 0 {
		t.Fatal("local parsing called upstream")
	}
}

func TestResourceValidationAndMirrorFailurePreserveSendingBoundary(t *testing.T) {
	f := newResourceServiceFixture(t)
	ctx := context.Background()
	kind := domain.ResourceTemplates
	f.rows[kind]["11"] = domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: "11", Name: "测试", Content: "主机{host}"}, AuditStatus: 2}
	rows, _ := f.s.Preview(ctx, f.account.ID, kind, "")
	if _, err := f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(rows[0], "create")}}); err != nil {
		t.Fatal(err)
	}
	var local model.ProviderTemplate
	f.s.db.First(&local)
	binding := model.ChannelTemplateBinding{ChannelID: 1, ProviderID: f.account.ID, ProviderTemplateID: local.ID, Status: 1, IsActive: 1, Weight: 1, MappedContentVersion: 1, ParamMapping: `[{"type":"fixed","provider_var":"host","value":"server"}]`}
	if err := f.s.db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	rows, _ = f.s.Preview(ctx, f.account.ID, kind, "11")
	req := ResourceMutationRequest{ResourceInput: domain.ResourceInput{ID: "11", Content: "新正文{host}", Description: "通知"}, Version: rows[0].Version, ConfirmImpact: true}
	if _, err := f.s.Mutate(ctx, f.account.ID, kind, domain.ResourceUpdate, req); err == nil {
		t.Fatal("invalid form accepted")
	}
	f.s.db.First(&local, local.ID)
	f.s.db.First(&binding, binding.ID)
	if !local.Usable() || binding.MappedContentVersion != 1 || f.writes != 0 {
		t.Fatal("invalid form suspended delivery")
	}
	req.Name = "测试"
	if err := f.s.db.Callback().Update().Before("gorm:update").Register("fail_mirror_body", func(tx *gorm.DB) {
		if updates, ok := tx.Statement.Dest.(map[string]any); ok && tx.Statement.Table == "provider_templates" {
			if _, changingBody := updates["template_content"]; changingBody {
				tx.AddError(errors.New("mirror save failed"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	result, err := f.s.Mutate(ctx, f.account.ID, kind, domain.ResourceUpdate, req)
	if err != nil || result == nil || !result.PendingSync || result.RemoteID != "11" || f.writes != 1 {
		t.Fatalf("partial success: %+v %v", result, err)
	}
	f.s.db.First(&local, local.ID)
	f.s.db.First(&binding, binding.ID)
	if local.Usable() || local.TemplateContent != "主机{host}" || binding.MappedContentVersion != 0 || binding.ParamMapping != "[]" {
		t.Fatal("old mapping survived failed mirror save")
	}
	if err := f.s.db.Callback().Update().Remove("fail_mirror_body"); err != nil {
		t.Fatal(err)
	}
	rows, _ = f.s.Preview(ctx, f.account.ID, kind, "11")
	if _, err = f.s.Import(ctx, f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(rows[0], "update")}}); err != nil {
		t.Fatal(err)
	}
	f.s.db.First(&local, local.ID)
	if local.ContentVersion != 2 || local.TemplateContent != "新正文{host}" {
		t.Fatal("query did not recover mirror")
	}
}
