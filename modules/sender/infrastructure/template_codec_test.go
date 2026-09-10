package infrastructure

import (
	"reflect"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func TestNativeTemplateParseBindAndRender(t *testing.T) {
	tests := []struct {
		provider, content, rendered, system string
		values                              map[string]string
		vars, tokens, order                 []string
	}{
		{constants.ProviderZrwinfoSMS, "主机${host_name}规则{rule_name}再次{host_name}", "主机$server规则cpu再次server", "主机${host_name}规则{rule_name}再次{host_name}", map[string]string{"host_name": "server", "rule_name": "cpu"}, []string{"host_name", "rule_name"}, []string{"{host_name}", "{rule_name}"}, []string{"server", "cpu", "server"}},
		{constants.ProviderZrwinfoSMS, "规则{2}主机{1}再次{1}", "规则cpu主机server再次server", "规则{2}主机{1}再次{1}", map[string]string{"1": "server", "2": "cpu"}, []string{"2", "1"}, []string{"{2}", "{1}"}, []string{"server", "cpu"}},
		{constants.ProviderAliyunSMS, "验证码${code}", "验证码123", "验证码{code}", map[string]string{"code": "123"}, []string{"code"}, []string{"${code}"}, []string{"123"}},
		{constants.ProviderTencentSMS, "规则{2}主机{1}", "规则cpu主机server", "规则{2}主机{1}", map[string]string{"1": "server", "2": "cpu"}, []string{"2", "1"}, []string{"{2}", "{1}"}, []string{"server", "cpu"}},
		{constants.ProviderNeteaseSMS, "主机%s规则%s", "主机server规则cpu", "主机{1}规则{2}", map[string]string{"1": "server", "2": "cpu"}, []string{"1", "2"}, []string{"%s"}, []string{"server", "cpu"}},
		{constants.ProviderNeteaseSMS, "{code}再次{code}", "123再次123", "{code}再次{code}", map[string]string{"code": "123"}, []string{"code"}, []string{"{code}"}, []string{"123", "123"}},
		{constants.ProviderZrwinfoSMS, "无变量正文", "无变量正文", "无变量正文", nil, []string{}, []string{}, []string{}},
	}
	for _, test := range tests {
		t.Run(test.provider+test.content, func(t *testing.T) {
			account := &model.ProviderAccount{ProviderCode: test.provider, ProviderType: "sms", Config: "{}"}
			parsed, err := domain.ParseAccountTemplate(account, test.content)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Content != test.content || parsed.SystemContent != test.system || !reflect.DeepEqual(parsed.Variables, test.vars) || !reflect.DeepEqual(parsed.NativeVariables, test.tokens) {
				t.Fatalf("parsed: %+v", parsed)
			}
			bound, err := parsed.Bind(test.values)
			if err != nil || !reflect.DeepEqual(bound.Ordered, test.order) {
				t.Fatalf("bind: %+v %v", bound, err)
			}
			rendered, err := parsed.Render(test.values)
			if err != nil || rendered != test.rendered {
				t.Fatalf("render: %q %v", rendered, err)
			}
			if len(test.vars) > 0 {
				if _, err = parsed.Bind(map[string]string{}); err == nil {
					t.Fatal("missing parameters accepted")
				}
			}
		})
	}
}

func TestNativeTemplateRejectsUnknownSyntax(t *testing.T) {
	for provider, contents := range map[string][]string{
		constants.ProviderZrwinfoSMS: {"", "{host}{1}", "{0}", "{01}", "{2}", "{1001}", "{host", "host}", "{{host}}", "{a.b}", "#host#"},
		constants.ProviderTencentSMS: {"{host}", "{1}{3}"},
		constants.ProviderAliyunSMS:  {"验证码{code}", "${1}"},
		constants.ProviderNeteaseSMS: {"%s与{name}"},
	} {
		for _, content := range contents {
			if _, err := domain.ParseAccountTemplate(&model.ProviderAccount{ProviderCode: provider, Config: "{}"}, content); err == nil {
				t.Errorf("accepted %s %q", provider, content)
			}
		}
	}
	if _, err := domain.ParseAccountTemplate(&model.ProviderAccount{ProviderCode: constants.ProviderNeteaseSMS, Config: `{"send_type":"code"}`}, "验证码%s"); err == nil {
		t.Fatal("anonymous code-mode token accepted")
	}
}

func confirmedSMSBinding(provider, content string) (*model.ProviderAccount, *model.ChannelTemplateBinding) {
	approved := int8(2)
	account := &model.ProviderAccount{ID: 1, ProviderCode: provider, ProviderType: "sms", Config: "{}", Status: 1}
	template := &model.ProviderTemplate{ID: 1, ProviderID: 1, TemplateCode: "11", TemplateContent: content, ContentVersion: 1, Status: 1, ProviderAccount: account, ProviderResourceState: model.ProviderResourceState{AuditStatus: &approved}}
	binding := &model.ChannelTemplateBinding{ProviderTemplate: template, ProviderTemplateID: 1, ProviderID: 1, MappedContentVersion: 1, Status: 1, IsActive: 1}
	return account, binding
}

func TestSMSParametersRequireConfirmationAndNativeKeys(t *testing.T) {
	account, binding := confirmedSMSBinding(constants.ProviderZrwinfoSMS, "主机{1}规则{2}")
	params := map[string]string{"1": "host", "2": "rule"}
	if _, err := smsTemplateParameters(account, binding, params); err != nil {
		t.Fatal(err)
	}
	binding.MappedContentVersion = 0
	if _, err := smsTemplateParameters(account, binding, params); err == nil {
		t.Fatal("unconfirmed binding accepted")
	}
	binding.MappedContentVersion = 1
	binding.ProviderTemplate.ContentVersion = 2
	if _, err := smsTemplateParameters(account, binding, params); err == nil {
		t.Fatal("stale mapping accepted")
	}
	binding.ProviderTemplate.ContentVersion = 1
	if _, err := smsTemplateParameters(account, binding, map[string]string{"var1": "host", "var2": "rule"}); err == nil {
		t.Fatal("legacy aliases accepted")
	}
	binding.ProviderTemplate.RemoteDeleted = true
	if _, err := smsTemplateParameters(account, binding, params); err == nil {
		t.Fatal("deleted resource accepted")
	}
}
