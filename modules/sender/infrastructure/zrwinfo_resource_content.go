package infrastructure

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func (s *ZrwinfoSMSSender) buildResourceContent(binding *model.ChannelTemplateBinding, legacyContent string, params map[string]string) (string, error) {
	if binding == nil || binding.ProviderTemplate == nil || binding.ProviderTemplate.CodecVersion == "" {
		return s.buildContentFromMapping(legacyContent, params), nil
	}
	t := binding.ProviderTemplate
	if !t.Usable() {
		return "", fmt.Errorf("供应商模板未审核通过或不可用")
	}
	meta, err := domain.GetByCode(s.GetProviderCode())
	if err != nil {
		return "", err
	}
	definition := meta.Resources[domain.ResourceTemplates]
	if definition == nil || definition.Codec == nil || definition.Codec.Version() != t.CodecVersion {
		return "", fmt.Errorf("不支持的模板转换版本")
	}
	var slots []domain.VariableSlot
	if err = json.Unmarshal([]byte(t.VariableSlots), &slots); err != nil {
		return "", fmt.Errorf("模板变量映射无效")
	}
	decoded, err := definition.Codec.Decode(t.NativeContent, slots)
	if err != nil || decoded.Content != t.TemplateContent || len(decoded.Slots) != len(slots) {
		return "", fmt.Errorf("模板原生内容与变量映射不一致")
	}
	values, err := definition.Codec.Bind(slots, params)
	if err != nil {
		return "", err
	}
	for _, value := range values {
		if utf8.RuneCountInString(value) < 1 || utf8.RuneCountInString(value) > 20 || strings.Contains(value, "##") || strings.Contains(value, "$$") {
			return "", fmt.Errorf("掌榕网变量须为 1 至 20 字符且不能包含参数分隔符")
		}
	}
	return strings.Join(values, "##"), nil
}
