package infrastructure

import (
	"reflect"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	"encoding/json"
)

func TestPositionalTemplateRoundTripAndRepeatedVariables(t *testing.T) {
	c := PositionalTemplateCodec{}
	p, err := c.Compile("验证码 {code}，{minutes} 分钟内有效，请填写 {code}")
	if err != nil {
		t.Fatal(err)
	}
	if p.NativeContent != "验证码 {1}，{2} 分钟内有效，请填写 {3}" || !reflect.DeepEqual(p.Variables, []string{"code", "minutes"}) {
		t.Fatalf("unexpected compile: %+v", p)
	}
	decoded, err := c.Decode(p.NativeContent, p.Slots)
	if err != nil || decoded.Content != p.Content {
		t.Fatalf("round trip: %+v %v", decoded, err)
	}
	values, err := c.Bind(p.Slots, map[string]string{"minutes": "5", "code": "123456"})
	if err != nil || !reflect.DeepEqual(values, []string{"123456", "5", "123456"}) {
		t.Fatalf("bind: %v %v", values, err)
	}
	if _, err = c.Bind(p.Slots, map[string]string{"code": "1"}); err == nil {
		t.Fatal("missing variable accepted")
	}
	fresh, err := c.Decode("{2} / {1} / {1}", nil)
	if err != nil || fresh.Content != "{var2} / {var1} / {var1}" {
		t.Fatalf("position order: %+v %v", fresh, err)
	}
}

func TestTemplateCodecRejectsUnknownOrIncompleteSyntax(t *testing.T) {
	c := PositionalTemplateCodec{}
	for _, value := range []string{"${code", "${code}}", "{{code}}", "#code#", "{ code }", "{中文}"} {
		if _, err := c.Compile(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	for _, value := range []string{"{2}", "{0}", "{1000000}", "{code}"} {
		if _, err := c.Decode(value, nil); err == nil {
			t.Errorf("accepted native %q", value)
		}
	}
	p, err := c.Compile("无变量通知")
	if err != nil || len(p.Slots) != 0 {
		t.Fatalf("static template: %v %v", p, err)
	}
}

func TestNamedTemplateCodecPreservesNativeNames(t *testing.T) {
	c := NamedTemplateCodec{Prefix: "$"}
	p, err := c.Compile("您的验证码是 {code}")
	if err != nil || p.NativeContent != "您的验证码是 ${code}" || !reflect.DeepEqual(p.NativeVariables, []string{"${code}"}) {
		t.Fatalf("compile: %+v %v", p, err)
	}
	d, err := c.Decode("您好 ${recipient}", []domain.VariableSlot{{Native: "recipient", Name: "name"}})
	if err != nil || d.Content != "您好 {name}" || d.Slots[0].Native != "recipient" || !reflect.DeepEqual(d.NativeVariables, []string{"${recipient}"}) {
		t.Fatalf("decode: %+v %v", d, err)
	}
}

func TestTemplateCodecsPreserveLiteralDollarText(t *testing.T) {
	const content = "价格$5，变量${code}，再取$$${code}"
	for _, codec := range []domain.TemplateCodec{PositionalTemplateCodec{}, NamedTemplateCodec{Prefix: "$"}} {
		t.Run(codec.Version(), func(t *testing.T) {
			compiled, err := codec.Compile(content)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := codec.Decode(compiled.NativeContent, compiled.Slots)
			if err != nil || decoded.Content != content {
				t.Fatalf("round trip lost literal text: %+v %v", decoded, err)
			}
		})
	}
}

func TestZrwinfoConvertedResourceContentUsesStoredSlots(t *testing.T) {
	p, _ := (PositionalTemplateCodec{}).Compile("验证码{code}，{minutes}分钟")
	slots, _ := json.Marshal(p.Slots)
	approved := int8(2)
	template := &model.ProviderTemplate{ProviderResourceState: model.ProviderResourceState{AuditStatus: &approved}, TemplateContent: p.Content, NativeContent: p.NativeContent, CodecVersion: p.Version, VariableSlots: string(slots), Status: 1}
	binding := &model.ChannelTemplateBinding{ProviderTemplate: template}
	s := &ZrwinfoSMSSender{}
	content, err := s.buildResourceContent(binding, "", map[string]string{"minutes": "5", "code": "123456"})
	if err != nil || content != "123456##5" {
		t.Fatalf("content=%q err=%v", content, err)
	}
	for _, value := range []string{"", "a##b", "a$$b", "123456789012345678901"} {
		if _, err = s.buildResourceContent(binding, "", map[string]string{"code": value, "minutes": "5"}); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
	approved = 1
	if _, err = s.buildResourceContent(binding, "", map[string]string{}); err == nil {
		t.Fatal("pending template was usable")
	}
}
