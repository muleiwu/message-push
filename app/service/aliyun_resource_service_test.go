package service

import (
	"context"
	"errors"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	"gorm.io/gorm"
)

type aliyunServiceFixture struct {
	*resourceServiceFixture
	writeError   error
	reflectWrite bool
}

func newAliyunServiceFixture(t *testing.T) *aliyunServiceFixture {
	t.Helper()
	f := &aliyunServiceFixture{resourceServiceFixture: newResourceServiceFixture(t)}
	f.account.ProviderCode = constants.ProviderAliyunSMS
	if err := f.s.db.Save(f.account).Error; err != nil {
		t.Fatal(err)
	}
	registered, err := domain.GetByCode(constants.ProviderAliyunSMS)
	if err != nil {
		t.Fatal(err)
	}
	meta := *registered
	meta.Resources = map[domain.ResourceKind]*domain.ResourceDefinition{}
	for kind, def := range registered.Resources {
		copyDef := *def
		copyDef.Operations = map[domain.ResourceAction]*domain.ResourceOperation{}
		for action, op := range def.Operations {
			copyOp := *op
			copyOp.Handler = func(_ context.Context, _ *model.ProviderAccount, input domain.ResourceInput) ([]domain.RemoteResource, error) {
				if action == domain.ResourceQuery {
					rows := []domain.RemoteResource{}
					for _, row := range f.rows[kind] {
						if input.ID == "" || row.ID == input.ID {
							rows = append(rows, row)
						}
					}
					return rows, nil
				}
				f.writes++
				if f.writeError != nil {
					return nil, f.writeError
				}
				id := input.ID
				if id == "" {
					id = input.Content
					if kind == domain.ResourceTemplates {
						id = "SMS_created"
					}
				}
				if f.reflectWrite {
					if action == domain.ResourceDelete {
						delete(f.rows[kind], id)
					} else {
						input = input.PublicCopy(op.Fields)
						input.ID = id
						f.rows[kind][id] = domain.RemoteResource{ResourceInput: input, AuditStatus: 1, ProviderMetadata: aliyunServiceMetadata("order-new")}
					}
				}
				return []domain.RemoteResource{{ResourceInput: domain.ResourceInput{ID: id}, ProviderMetadata: aliyunServiceMetadata("order-new"), RequestID: "request-write"}}, nil
			}
			copyDef.Operations[action] = &copyOp
		}
		meta.Resources[kind] = &copyDef
	}
	f.s.lookup = func(string) (*domain.ProviderMeta, error) { return &meta, nil }
	return f
}

func aliyunServiceMetadata(order string) map[string]any {
	return map[string]any{"aliyun": map[string]any{"order_id": order}}
}

func aliyunServiceInput(kind domain.ResourceKind) domain.ResourceInput {
	if kind == domain.ResourceTemplates {
		return domain.ResourceInput{ID: "SMS_123", Name: "用户通知", Content: "用户${name}你好", Category: "1", ProviderFields: map[string]string{"related_sign_name": "木雷科技", "template_rule": `{"name":"character"}`, "apply_scene_content": "用户通知"}}
	}
	return domain.ResourceInput{ID: "木雷科技", Content: "木雷科技", ProviderFields: map[string]string{"qualification_id": "1234", "sign_type": "1", "sign_source": "0"}}
}

func (f *aliyunServiceFixture) preview(t *testing.T, kind domain.ResourceKind) *ResourcePreviewItem {
	t.Helper()
	rows, err := f.s.Preview(context.Background(), f.account.ID, kind, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("preview: %+v %v", rows, err)
	}
	return rows[0]
}

func (f *aliyunServiceFixture) importRow(t *testing.T, kind domain.ResourceKind, action string) {
	t.Helper()
	row := f.preview(t, kind)
	_, err := f.s.Import(context.Background(), f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(row, action)}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAliyunRestrictedMutationDoesNotSuspendLocalTemplate(t *testing.T) {
	f := newAliyunServiceFixture(t)
	kind := domain.ResourceTemplates
	input := aliyunServiceInput(kind)
	f.rows[kind][input.ID] = domain.RemoteResource{ResourceInput: input, AuditStatus: 2, ProviderMetadata: aliyunServiceMetadata("order-old")}
	f.importRow(t, kind, "create")
	var local model.ProviderTemplate
	f.s.db.First(&local)
	binding := model.ChannelTemplateBinding{ChannelID: 1, ProviderID: f.account.ID, ProviderTemplateID: local.ID, MappedContentVersion: 1, ParamMapping: `[{"type":"fixed","provider_var":"name","value":"用户"}]`, Status: 1, IsActive: 1, Weight: 1}
	if err := f.s.db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	before := f.preview(t, kind)
	if before.OperationErrors[domain.ResourceUpdate] == "" {
		t.Fatal("approved template advertised editing")
	}
	input.Content = "尊敬的${name}你好"
	_, err := f.s.Mutate(context.Background(), f.account.ID, kind, domain.ResourceUpdate, ResourceMutationRequest{ResourceInput: input, Version: before.Version, ConfirmImpact: true})
	if err == nil {
		t.Fatal("approved template updated")
	}
	f.s.db.First(&local, local.ID)
	f.s.db.First(&binding, binding.ID)
	if !local.Usable() || binding.MappedContentVersion != 1 || binding.ParamMapping == "[]" || f.writes != 0 {
		t.Fatal("rejected preflight changed delivery state")
	}
}

func TestAliyunLegacySignatureImportAndStaleReviewProtection(t *testing.T) {
	for _, failure := range []string{"stale", "uncertain", "rejected", "mirror"} {
		t.Run(failure, func(t *testing.T) {
			f := newAliyunServiceFixture(t)
			kind := domain.ResourceSignatures
			input := aliyunServiceInput(kind)
			old := domain.RemoteResource{ResourceInput: input, AuditStatus: 2, ProviderMetadata: aliyunServiceMetadata("order-old")}
			f.rows[kind][input.ID] = old
			local := model.ProviderSignature{ProviderAccountID: f.account.ID, SignatureName: "本地名称", SignatureCode: input.Content, Status: 1}
			if err := f.s.db.Create(&local).Error; err != nil {
				t.Fatal(err)
			}
			f.importRow(t, kind, "update")
			f.s.db.First(&local, local.ID)
			if local.RemoteID != input.Content || local.SignatureName != "本地名称" {
				t.Fatal("legacy signature identity was lost")
			}
			before := f.preview(t, kind)
			switch failure {
			case "uncertain":
				f.writeError = &domain.RemoteResourceError{Code: "TRANSPORT_ERROR", Uncertain: true}
			case "rejected":
				f.writeError = &domain.RemoteResourceError{Code: "InvalidQualification", Uncertain: false}
			case "mirror":
				if err := f.s.db.Callback().Update().Before("gorm:update").Register("fail_aliyun_mirror", func(tx *gorm.DB) {
					if updates, ok := tx.Statement.Dest.(map[string]any); ok && tx.Statement.Table == "provider_signatures" {
						if _, changing := updates["signature_code"]; changing {
							tx.AddError(errors.New("save failed"))
						}
					}
				}); err != nil {
					t.Fatal(err)
				}
			}
			result, err := f.s.Mutate(context.Background(), f.account.ID, kind, domain.ResourceUpdate, ResourceMutationRequest{ResourceInput: input, Version: before.Version, ConfirmImpact: true})
			if failure == "stale" || failure == "mirror" {
				if err != nil || result == nil || result.RequestID != "request-write" || result.PendingSync != (failure == "mirror") {
					t.Fatalf("mutation: %+v %v", result, err)
				}
			} else if err == nil {
				t.Fatal("lost write failure")
			}
			if failure == "mirror" {
				if err := f.s.db.Callback().Update().Remove("fail_aliyun_mirror"); err != nil {
					t.Fatal(err)
				}
			}
			f.s.db.First(&local, local.ID)
			if local.Usable() || f.writes != 1 {
				t.Fatal("old signature remains usable or write was retried")
			}
			row := f.preview(t, kind)
			if failure == "rejected" {
				if row.Error != "" {
					t.Fatal("definite rejection prevented recovery")
				}
			} else {
				if row.Error == "" || row.Remote.AuditStatus != 0 {
					t.Fatal("old review was accepted")
				}
				_, err := f.s.Import(context.Background(), f.account.ID, ResourceImportRequest{Kind: kind, Selections: []ResourceSelection{resourceSelection(row, "update")}})
				if err == nil {
					t.Fatal("old review restored usable signature")
				}
				old.ProviderMetadata = aliyunServiceMetadata("order-new")
				f.rows[kind][input.ID] = old
			}
			f.importRow(t, kind, "update")
			// Use a fresh value: GORM does not clear absent JSON map keys in a reused destination.
			var recovered model.ProviderSignature
			f.s.db.First(&recovered, local.ID)
			if !recovered.Usable() || recovered.ProviderMetadata[pendingResourceReview] != nil {
				t.Fatalf("review not recovered: %+v", recovered)
			}
		})
	}
}

func TestAliyunTemplateContentChangeResetsMappingOnce(t *testing.T) {
	f := newAliyunServiceFixture(t)
	f.reflectWrite = true
	kind := domain.ResourceTemplates
	input := aliyunServiceInput(kind)
	f.rows[kind][input.ID] = domain.RemoteResource{ResourceInput: input, AuditStatus: 3, ProviderMetadata: aliyunServiceMetadata("order-old")}
	f.importRow(t, kind, "create")
	var local model.ProviderTemplate
	f.s.db.First(&local)
	binding := model.ChannelTemplateBinding{ChannelID: 1, ProviderID: f.account.ID, ProviderTemplateID: local.ID, MappedContentVersion: 1, ParamMapping: `[{"type":"fixed","provider_var":"name","value":"用户"}]`, Status: 1, IsActive: 1, Weight: 1}
	if err := f.s.db.Create(&binding).Error; err != nil {
		t.Fatal(err)
	}
	before := f.preview(t, kind)
	input.Content = "尊敬的${name}你好"
	_, err := f.s.Mutate(context.Background(), f.account.ID, kind, domain.ResourceUpdate, ResourceMutationRequest{ResourceInput: input, Version: before.Version, ConfirmImpact: true})
	if err != nil {
		t.Fatal(err)
	}
	f.s.db.First(&local, local.ID)
	f.s.db.First(&binding, binding.ID)
	if local.ContentVersion != 2 || local.TemplateContent != input.Content || local.Usable() || binding.MappedContentVersion != 0 || binding.ParamMapping != "[]" {
		t.Fatalf("mapping not reset: %+v %+v", local, binding)
	}
	if err := f.s.db.Model(&binding).Updates(map[string]any{"mapped_content_version": 2, "param_mapping": `[{"type":"fixed","provider_var":"name","value":"用户"}]`}).Error; err != nil {
		t.Fatal(err)
	}
	row := f.rows[kind][input.ID]
	row.AuditStatus = 2
	f.rows[kind][input.ID] = row
	f.importRow(t, kind, "update")
	f.s.db.First(&local, local.ID)
	f.s.db.First(&binding, binding.ID)
	if !local.Usable() || local.ContentVersion != 2 || binding.MappedContentVersion != 2 || binding.ParamMapping == "[]" {
		t.Fatal("audit-only sync reset confirmed mapping")
	}
}
