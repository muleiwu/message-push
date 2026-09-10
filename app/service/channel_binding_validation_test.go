package service

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/readiness"
)

func TestChannelBindingValidationErrorPreservesReadinessCodes(t *testing.T) {
	status := int8(1)
	binding := &model.ChannelTemplateBinding{
		Status: 1, IsActive: 1, Weight: 10, ProviderID: 1,
		ProviderTemplate: &model.ProviderTemplate{
			ProviderResourceState: model.ProviderResourceState{AuditStatus: &status},
			Status:                1, ProviderID: 1, TemplateCode: "3344803", Variables: `["host_name"]`,
			ProviderAccount: &model.ProviderAccount{ID: 1, ProviderCode: constants.ProviderZrwinfoSMS, ProviderType: "sms", Status: 1},
		},
		ParamMapping: `[{"type":"mapping","provider_var":"host_name","system_var":"missing"}]`,
	}
	codes := readiness.ValidateBinding("sms", []string{"host_name"}, binding)
	err := newChannelBindingValidationError(binding, codes)
	var validationErr *ChannelBindingValidationError
	if !errors.As(fmt.Errorf("wrapped: %w", err), &validationErr) {
		t.Fatal("wrapped binding validation error lost its type")
	}
	if !reflect.DeepEqual(validationErr.Codes, codes) {
		t.Fatalf("readiness codes changed: %v != %v", validationErr.Codes, codes)
	}
	want := "参数映射无效或不完整，请检查供应商变量与系统变量的对应关系；供应商模板尚未审核通过，请审核通过并同步后重试"
	if err.Error() != want {
		t.Fatalf("message = %q, want %q", err.Error(), want)
	}
	// Repeated stable aliases must not repeat either the code or the explanation.
	duplicate := newChannelBindingValidationError(binding, append(append([]string(nil), codes...), codes...))
	if !reflect.DeepEqual(duplicate.Codes, codes) || duplicate.Message != want {
		t.Fatalf("duplicate issues were not collapsed: %+v", duplicate)
	}
}
