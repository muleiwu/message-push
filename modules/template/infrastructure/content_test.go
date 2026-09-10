package infrastructure

import (
	"reflect"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/template/domain"
)

func TestPrepareContentMatchesSendMappingSemantics(t *testing.T) {
	tests := []struct {
		name, raw, mapping, variables, content, wantContent string
		wantParams                                          map[string]string
		wantMissing, wantUnavailable                        bool
	}{
		{
			name:    "explicit rename and fixed value",
			raw:     `{"code":"086697","expire_time":"10","internal":"unused"}`,
			mapping: `[{"type":"mapping","provider_var":"var1","system_var":"code"},{"type":"fixed","provider_var":"ttl","value":"5"}]`,
			content: "验证码 {var1}，{ttl} 分钟内有效。", wantContent: "验证码 086697，5 分钟内有效。",
			wantParams: map[string]string{"var1": "086697", "ttl": "5"},
		},
		{
			name: "same-name subset preserves empty and zero",
			raw:  `{"code":"0","empty":"","internal":"unused"}`, variables: `["code","empty"]`,
			content: "{code}/{empty}", wantContent: "0/", wantParams: map[string]string{"code": "0", "empty": ""},
		},
		{
			name: "missing explicit source is sent as empty",
			raw:  `{}`, mapping: `[{"type":"mapping","provider_var":"var1","system_var":"code"}]`,
			content: "验证码{var1}", wantContent: "验证码", wantParams: map[string]string{"var1": ""}, wantMissing: true,
		},
		{
			name: "missing same-name source is not forwarded",
			raw:  `{}`, variables: `["code"]`, content: "验证码{code}", wantContent: "验证码{code}",
			wantParams: map[string]string{}, wantMissing: true,
		},
		{
			name: "fixed empty string with null original params",
			raw:  `null`, mapping: `[{"type":"fixed","provider_var":"code","value":""}]`,
			content: "验证码{code}", wantContent: "验证码", wantParams: map[string]string{"code": ""},
		},
		{
			name:    "empty raw params retain legacy skip behavior",
			mapping: `[{"type":"fixed","provider_var":"code","value":"1234"}]`,
			content: "验证码{code}", wantContent: "验证码{code}",
		},
		{
			name: "invalid original params", raw: `{"code":123}`, variables: `["code"]`,
			content: "验证码{code}", wantContent: "验证码{code}", wantMissing: true, wantUnavailable: true,
		},
		{
			name: "invalid mapping", raw: `{"code":"1234"}`, mapping: `{invalid`,
			content: "验证码{code}", wantContent: "验证码{code}", wantUnavailable: true,
		},
		{
			name: "static template without params", content: "发送成功", wantContent: "发送成功",
		},
		{
			name: "markup remains literal", raw: `{"body":"<b>原文</b>"}`, variables: `["body"]`,
			content: "<p>{body}</p>\n**正文**", wantContent: "<p><b>原文</b></p>\n**正文**", wantParams: map[string]string{"body": "<b>原文</b>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binding := &model.ChannelTemplateBinding{ID: 7, ParamMapping: tt.mapping, ProviderTemplate: &model.ProviderTemplate{
				ID: 9, TemplateCode: "provider-template", TemplateName: "验证码", TemplateContent: tt.content, Variables: tt.variables, ContentType: "text",
			}}
			got := domain.PrepareContent(NewTemplateHelper(), tt.raw, binding)
			if got.Content != tt.wantContent || !reflect.DeepEqual(got.MappedParams, tt.wantParams) {
				t.Fatalf("content=%q params=%#v, want %q %#v", got.Content, got.MappedParams, tt.wantContent, tt.wantParams)
			}
			if (got.UnavailableReason != "") != tt.wantUnavailable {
				t.Fatalf("unavailable reason = %q", got.UnavailableReason)
			}
			if got.BindingID != 7 || got.ProviderTemplateID != 9 || got.OriginalParamsRaw != tt.raw {
				t.Fatalf("lost template or input metadata: %+v", got)
			}
			missing := false
			for _, row := range got.MappingDetails {
				missing = missing || row.Missing
				value, exists := got.MappedParams[row.ProviderVar]
				if (row.Value != nil) != exists || (exists && *row.Value != value) {
					t.Fatalf("mapping row does not describe sender input: %+v", row)
				}
			}
			if missing != tt.wantMissing {
				t.Fatalf("missing source = %v, want %v", missing, tt.wantMissing)
			}
		})
	}
}

func TestPrepareContentUnavailableTemplate(t *testing.T) {
	for _, binding := range []*model.ChannelTemplateBinding{nil, {}, {ProviderTemplate: &model.ProviderTemplate{}}} {
		got := domain.PrepareContent(NewTemplateHelper(), `{"code":"1234"}`, binding)
		if got.UnavailableReason == "" {
			t.Fatal("expected missing template/body reason")
		}
	}
}
