package infrastructure

import (
	"fmt"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func smsTemplateParameters(account *model.ProviderAccount, binding *model.ChannelTemplateBinding, values map[string]string) (*domain.TemplateParameters, error) {
	if account == nil || binding == nil || binding.ProviderTemplate == nil {
		return nil, fmt.Errorf("短信发送需要已确认的供应商模板绑定")
	}
	t := binding.ProviderTemplate
	if !t.Usable() || binding.Status != 1 || binding.IsActive != 1 || t.ContentVersion == 0 || binding.MappedContentVersion != t.ContentVersion {
		return nil, fmt.Errorf("短信模板不可用或参数映射尚未确认")
	}
	if t.TemplateCode == "" || t.ProviderID != account.ID || binding.ProviderID != account.ID {
		return nil, fmt.Errorf("短信模板与供应商账号不匹配")
	}
	parsed, err := domain.ParseAccountTemplate(account, t.TemplateContent)
	if err != nil {
		return nil, err
	}
	return parsed.Bind(values)
}
