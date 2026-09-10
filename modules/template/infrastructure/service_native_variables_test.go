package infrastructure

import (
	"reflect"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	_ "cnb.cool/mliev/push/message-push/modules/sender/infrastructure"
)

func TestProviderTemplateResponseSeparatesNativeTokensFromMappingNames(t *testing.T) {
	account := &model.ProviderAccount{ID: 1, ProviderCode: constants.ProviderZrwinfoSMS, ProviderType: "sms"}
	for _, test := range []struct {
		name, native, content string
		variables, tokens     []string
	}{
		{"numeric", "主机{1}告警，使用率{2}%", "主机{host_name}告警，使用率{usage}%", []string{"host_name", "usage"}, []string{"{1}", "{2}"}},
		{"named", "主机{host_name}规则{rule_name}恢复。", "主机{host_name}规则{rule_name}恢复。", []string{"host_name", "rule_name"}, []string{"{host_name}", "{rule_name}"}},
		{"literal-dollar", "主机${host_name}规则{rule_name}恢复。", "主机${host_name}规则{rule_name}恢复。", []string{"host_name", "rule_name"}, []string{"{host_name}", "{rule_name}"}},
		{"manual", "", "主机{host_name}告警。", []string{"host_name"}, []string{"{host_name}"}},
		{"static", "固定内容。", "固定内容。", []string{}, []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := &model.ProviderTemplate{ProviderID: account.ID, ProviderAccount: account, NativeContent: test.native, TemplateContent: test.content}
			if err := record.SetVariables(test.variables); err != nil {
				t.Fatal(err)
			}
			response, err := (&TemplateService{}).buildProviderTemplateResponse(record)
			if err != nil {
				t.Fatal(err)
			}
			if response.NativeContent != test.native || response.TemplateContent != test.content || !reflect.DeepEqual(response.NativeVariables, test.tokens) || !reflect.DeepEqual(response.Variables, test.variables) {
				t.Fatalf("unexpected response: %+v", response)
			}
		})
	}
}

func TestProviderTemplateResponseKeepsInvalidOriginalReadable(t *testing.T) {
	record := &model.ProviderTemplate{NativeContent: "主机{host_name}规则{1}", TemplateContent: "原映射内容", ProviderAccount: &model.ProviderAccount{ProviderCode: constants.ProviderZrwinfoSMS}}
	response, err := (&TemplateService{}).buildProviderTemplateResponse(record)
	if err != nil || response.NativeContent != record.NativeContent || response.NativeVariables != nil {
		t.Fatalf("invalid original should remain available: %+v %v", response, err)
	}
}
