package infrastructure

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"cnb.cool/mliev/push/message-push/app/model"
)

func (s *ZrwinfoSMSSender) buildResourceContent(binding *model.ChannelTemplateBinding, account *model.ProviderAccount, params map[string]string) (string, error) {
	bound, err := smsTemplateParameters(account, binding, params)
	if err != nil {
		return "", err
	}
	for _, value := range bound.Ordered {
		if utf8.RuneCountInString(value) < 1 || utf8.RuneCountInString(value) > 20 || strings.Contains(value, "##") || strings.Contains(value, "$$") {
			return "", fmt.Errorf("掌榕网变量须为 1 至 20 字符且不能包含参数分隔符")
		}
	}
	return strings.Join(bound.Ordered, "##"), nil
}
