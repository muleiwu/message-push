package infrastructure

import (
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func TestZrwinfoScreenshotAndDollarTemplates(t *testing.T) {
	for _, content := range []string{
		"主机{host_name}的监控规则“{rule_name}”已满足恢复条件，本次告警已恢复，请关注后续运行状态。",
		"主机{host_name}的监控规则“{rule_name}”触发探测失败告警，请及时检查目标服务及网络连接。",
		"主机{host_name}触发失联告警，请及时检查主机运行状态、网络连接及监控程序。",
		"主机${host_name}的监控规则“{rule_name}”触发时延告警，当前时延{value}毫秒，告警阈值{threshold}毫秒，请及时排查。",
	} {
		t.Run(content, func(t *testing.T) {
			account, binding := confirmedSMSBinding(constants.ProviderZrwinfoSMS, content)
			parsed, err := domain.ParseProviderTemplate(binding.ProviderTemplate)
			if err != nil || parsed.Content != content {
				t.Fatalf("parse: %+v %v", parsed, err)
			}
			params := map[string]string{"host_name": "server01", "rule_name": "latency", "value": "500", "threshold": "200"}
			want := []string{}
			for _, key := range parsed.Variables {
				want = append(want, params[key])
			}
			got, err := NewZrwinfoSMSSender().buildResourceContent(binding, account, params)
			if err != nil || got != strings.Join(want, "##") {
				t.Fatalf("send: %q %v", got, err)
			}
		})
	}
}

func TestZrwinfoRejectsUnsafeParameterSeparators(t *testing.T) {
	account, binding := confirmedSMSBinding(constants.ProviderZrwinfoSMS, "验证码{code}")
	for _, value := range []string{"", "a##b", "a$$b", strings.Repeat("长", 21)} {
		if _, err := NewZrwinfoSMSSender().buildResourceContent(binding, account, map[string]string{"code": value}); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}
