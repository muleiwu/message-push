package infrastructure

import (
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	templatedomain "cnb.cool/mliev/push/message-push/modules/template/domain"
)

func TestNativeSMSPreparationUsesExplicitMappings(t *testing.T) {
	for _, test := range []struct {
		provider, body, mapping, params, want string
		invalid                               bool
	}{
		{constants.ProviderZrwinfoSMS, "主机${host}", `[{"type":"mapping","provider_var":"host","system_var":"hostname"}]`, `{"hostname":"server"}`, "主机$server", false},
		{constants.ProviderAliyunSMS, "验证码${code}", `[{"type":"fixed","provider_var":"code","value":"123"}]`, "", "验证码123", false},
		{constants.ProviderTencentSMS, "规则{2}主机{1}", `[{"type":"fixed","provider_var":"1","value":"server"},{"type":"fixed","provider_var":"2","value":"cpu"}]`, "", "规则cpu主机server", false},
		{constants.ProviderNeteaseSMS, "主机%s规则%s", `[{"type":"fixed","provider_var":"1","value":"server"},{"type":"fixed","provider_var":"2","value":"cpu"}]`, "", "主机server规则cpu", false},
		{constants.ProviderZrwinfoSMS, "静态通知", "[]", "", "静态通知", false},
		{constants.ProviderZrwinfoSMS, "验证码{code}", "[]", `{"code":"123"}`, "", true},
		{constants.ProviderZrwinfoSMS, "验证码{code}", `[{"type":"mapping","provider_var":"code","system_var":"missing"}]`, `{}`, "", true},
		{constants.ProviderZrwinfoSMS, "验证码{code}", `[{"type":"fixed","provider_var":"code","value":"123"},{"type":"fixed","provider_var":"code","value":"456"}]`, "", "", true},
	} {
		t.Run(test.provider+test.body+test.mapping, func(t *testing.T) {
			account := &model.ProviderAccount{ProviderCode: test.provider, ProviderType: "sms", Config: "{}"}
			b := &model.ChannelTemplateBinding{MappedContentVersion: 1, ParamMapping: test.mapping, ProviderTemplate: &model.ProviderTemplate{ContentVersion: 1, TemplateContent: test.body, Variables: `["ignored"]`, ProviderAccount: account}}
			got := templatedomain.PrepareContent(NewTemplateHelper(), test.params, b)
			if (got.UnavailableReason != "") != test.invalid || got.Content != test.want {
				t.Fatalf("prepared: %+v", got)
			}
		})
	}
}
