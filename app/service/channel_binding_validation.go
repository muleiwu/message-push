package service

import (
	"fmt"
	"strings"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/readiness"
)

// ChannelBindingValidationError carries stable readiness codes alongside an
// actionable admin message. Delivery eligibility remains owned by readiness.
type ChannelBindingValidationError struct {
	Codes   []string
	Message string
}

func (e *ChannelBindingValidationError) Error() string { return e.Message }

func newChannelBindingValidationError(binding *model.ChannelTemplateBinding, codes []string) *ChannelBindingValidationError {
	result := &ChannelBindingValidationError{}
	var messages []string
	seenCodes := make(map[string]bool)
	seenMessages := make(map[string]bool)
	for _, code := range codes {
		if seenCodes[code] {
			continue
		}
		seenCodes[code] = true
		result.Codes = append(result.Codes, code)
		var descriptions []string
		switch code {
		case constants.ReadinessBlockerProviderTemplateMissing:
			// Several template failures deliberately share this stable public code.
			descriptions = describeUnavailableBindingTemplate(binding)
		case constants.ReadinessBlockerMappingUnconfirmed:
			descriptions = []string{"模板正文已变化，请重新配置并确认参数映射"}
		case constants.ReadinessBlockerParamMappingInvalid:
			descriptions = []string{"参数映射无效或不完整，请检查供应商变量与系统变量的对应关系"}
		case constants.ReadinessBlockerProviderAccountMissing:
			descriptions = []string{"供应商账号不可用，请检查账号状态与供应商配置"}
		case constants.ReadinessBlockerProviderAccountMismatch:
			descriptions = []string{"供应商模板、账号与通道类型不匹配，请检查绑定关系"}
		case constants.ReadinessBlockerBindingWeightInvalid:
			descriptions = []string{"绑定权重必须大于 0"}
		case constants.ReadinessBlockerBindingDisabled:
			descriptions = []string{"绑定已禁用，请启用后重试"}
		case constants.ReadinessBlockerBindingInactive:
			descriptions = []string{"绑定已熔断，请恢复后重试"}
		default:
			descriptions = []string{"通道绑定配置无效：" + code}
		}
		for _, description := range descriptions {
			if !seenMessages[description] {
				seenMessages[description] = true
				messages = append(messages, description)
			}
		}
	}
	result.Message = strings.Join(messages, "；")
	return result
}

// Explain only failures checked for this candidate: disabled bindings validate
// mappings without requiring an enabled, approved or numbered template.
func describeUnavailableBindingTemplate(binding *model.ChannelTemplateBinding) []string {
	if binding == nil || binding.ProviderTemplate == nil {
		return []string{"供应商模板不存在或已删除，请更换模板"}
	}
	template := binding.ProviderTemplate
	var messages []string
	if binding.Status == 1 && binding.IsActive == 1 {
		if template.Status != 1 {
			messages = append(messages, "供应商模板已禁用，请启用后重试")
		}
		if template.RemoteDeleted {
			messages = append(messages, "供应商模板已在远端删除，请更换模板")
		}
		if template.AuditStatus != nil {
			switch *template.AuditStatus {
			case 0:
				messages = append(messages, "供应商模板审核状态待确认，请同步审核结果后重试")
			case 1:
				if template.ProviderAccount != nil && template.ProviderAccount.ProviderCode == constants.ProviderTencentSMS && fmt.Sprint(template.ProviderMetadata["status_code"]) == "2" {
					messages = append(messages, "供应商模板审核通过但尚未生效，请生效并同步后重试")
				} else {
					messages = append(messages, "供应商模板尚未审核通过，请审核通过并同步后重试")
				}
			case 2:
				// Approved, including when another template check failed.
			case 3:
				messages = append(messages, "供应商模板审核未通过，请修改并重新提交审核")
			default:
				messages = append(messages, "供应商模板审核状态异常，请同步审核结果后重试")
			}
		}
		if strings.TrimSpace(template.TemplateCode) == "" {
			messages = append(messages, "供应商模板编号为空，请完善模板配置")
		}
	}
	if !readiness.ProviderTemplateVariablesValid(template) {
		messages = append(messages, "供应商模板变量定义异常，请修正配置或重新同步模板")
	}
	if len(messages) == 0 {
		messages = append(messages, "供应商模板不可用，请检查模板配置")
	}
	return messages
}
