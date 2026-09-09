package infrastructure

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func registeredZrwinfoCodec(t *testing.T) domain.TemplateCodec {
	t.Helper()
	meta, err := domain.GetByCode(constants.ProviderZrwinfoSMS)
	if err != nil {
		t.Fatal(err)
	}
	return meta.Resources[domain.ResourceTemplates].Codec
}

func TestZrwinfoScreenshotNamedPlaceholders(t *testing.T) {
	codec := registeredZrwinfoCodec(t)
	for _, content := range []string{
		"主机{host_name}的监控规则“{rule_name}”已满足恢复条件，本次告警已恢复，请关注后续运行状态。",
		"主机{host_name}的监控规则“{rule_name}”触发探测失败告警，请及时检查目标服务及网络连接。",
		"主机{host_name}触发失联告警，请及时检查主机运行状态、网络连接及监控程序。",
	} {
		t.Run(content, func(t *testing.T) {
			decoded, err := codec.Decode(content, nil)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Content != content || decoded.NativeContent != content {
				t.Fatalf("original content changed: %+v", decoded)
			}
			wantVariables := []string{"host_name"}
			wantTokens := []string{"{host_name}"}
			wantSend := "server01"
			if strings.Contains(content, "{rule_name}") {
				wantVariables = append(wantVariables, "rule_name")
				wantTokens = append(wantTokens, "{rule_name}")
				wantSend += "##health"
			}
			if !reflect.DeepEqual(decoded.Variables, wantVariables) || !reflect.DeepEqual(decoded.NativeVariables, wantTokens) {
				t.Fatalf("wrong variables: %+v", decoded)
			}
			encoded, err := json.Marshal(decoded.Slots)
			if err != nil {
				t.Fatal(err)
			}
			approved := int8(2)
			template := &model.ProviderTemplate{ProviderResourceState: model.ProviderResourceState{AuditStatus: &approved}, Status: 1, TemplateContent: decoded.Content, NativeContent: decoded.NativeContent, CodecVersion: decoded.Version, VariableSlots: string(encoded)}
			got, err := (&ZrwinfoSMSSender{}).buildResourceContent(&model.ChannelTemplateBinding{ProviderTemplate: template}, "", map[string]string{"host_name": "server01", "rule_name": "health"})
			if err != nil || got != wantSend {
				t.Fatalf("send=%q err=%v", got, err)
			}
		})
	}
}

func TestZrwinfoNamedOrderRepeatsAndNumericCompatibility(t *testing.T) {
	codec := registeredZrwinfoCodec(t)
	previous := []domain.VariableSlot{{Native: "1", Name: "host_name"}, {Native: "2", Name: "rule_name"}}
	decoded, err := codec.Decode("规则{rule_name}，主机{host_name}，再检查{rule_name}", previous)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.NativeVariables, []string{"{rule_name}", "{host_name}"}) {
		t.Fatalf("display tokens: %v", decoded.NativeVariables)
	}
	values, err := codec.Bind(decoded.Slots, map[string]string{"rule_name": "health", "host_name": "server01"})
	if err != nil || strings.Join(values, "##") != "health##server01##health" {
		t.Fatalf("named slots were renamed by old positions: %v %v", values, err)
	}
	if _, err = codec.Bind(decoded.Slots, map[string]string{"host_name": "server01"}); err == nil {
		t.Fatal("missing parameter accepted")
	}
	decoded, err = codec.Decode("先{2}，后{1}，再{2}", previous)
	if err != nil || decoded.Content != "先{rule_name}，后{host_name}，再{rule_name}" || decoded.Version != "positional-v1" {
		t.Fatalf("numeric compatibility: %+v %v", decoded, err)
	}
	if !reflect.DeepEqual(decoded.NativeVariables, []string{"{2}", "{1}"}) {
		t.Fatalf("native display order: %v", decoded.NativeVariables)
	}
	values, err = codec.Bind(decoded.Slots, map[string]string{"rule_name": "health", "host_name": "server01"})
	if err != nil || strings.Join(values, "##") != "server01##health" {
		t.Fatalf("numeric index order changed: %v %v", values, err)
	}
	compiled, err := codec.Compile("主机{host_name}的规则{rule_name}")
	if err != nil || compiled.NativeContent != "主机{1}的规则{2}" {
		t.Fatalf("submission rules changed: %+v %v", compiled, err)
	}
	if !reflect.DeepEqual(compiled.NativeVariables, []string{"{1}", "{2}"}) {
		t.Fatalf("compile tokens: %v", compiled.NativeVariables)
	}
}

func TestZrwinfoNativeSyntaxValidation(t *testing.T) {
	codec := registeredZrwinfoCodec(t)
	for _, content := range []string{"主机{host_name}规则{1}", "${host_name}", "{{host_name}}", "{中文}", "{ host_name }", "{host_name", "#host_name#", "{0}", "{2}"} {
		if _, err := codec.Decode(content, nil); err == nil {
			t.Errorf("accepted invalid native syntax %q", content)
		}
	}
	if _, err := (PositionalTemplateCodec{}).Decode("主机{host_name}", nil); err == nil {
		t.Fatal("default positional codec became permissive")
	}
	decoded, err := codec.Decode("无变量的中文通知。", nil)
	if err != nil || decoded.NativeVariables == nil || len(decoded.NativeVariables) != 0 {
		t.Fatalf("static template: %+v %v", decoded, err)
	}
}
