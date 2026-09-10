package infrastructure

import (
	"reflect"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	_ "cnb.cool/mliev/push/message-push/modules/sender/infrastructure"
)

func TestProviderTemplateResponseDerivesNativeVariables(t *testing.T) {
	for _, test := range []struct {
		content      string
		vars, tokens []string
	}{
		{"主机{host_name}规则{rule_name}", []string{"host_name", "rule_name"}, []string{"{host_name}", "{rule_name}"}},
		{"主机{1}规则{2}", []string{"1", "2"}, []string{"{1}", "{2}"}},
		{"通知", []string{}, []string{}},
	} {
		record := &model.ProviderTemplate{TemplateContent: test.content, Variables: `["obsolete_alias"]`, ContentVersion: 3, ProviderAccount: &model.ProviderAccount{ProviderCode: constants.ProviderZrwinfoSMS, ProviderType: "sms"}}
		response, err := (&TemplateService{}).buildProviderTemplateResponse(record)
		if err != nil || response.TemplateContent != test.content || response.ContentVersion != 3 || !reflect.DeepEqual(response.Variables, test.vars) || !reflect.DeepEqual(response.NativeVariables, test.tokens) {
			t.Fatalf("response: %+v %v", response, err)
		}
	}
}
func TestProviderTemplateResponseKeepsInvalidOriginalReadable(t *testing.T) {
	record := &model.ProviderTemplate{TemplateContent: "主机{host_name}规则{1}", ProviderAccount: &model.ProviderAccount{ProviderCode: constants.ProviderZrwinfoSMS, ProviderType: "sms"}}
	response, err := (&TemplateService{}).buildProviderTemplateResponse(record)
	if err != nil || response.TemplateContent != record.TemplateContent || response.ParseError == "" {
		t.Fatalf("response: %+v %v", response, err)
	}
}
