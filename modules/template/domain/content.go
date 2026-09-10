package domain

import (
	"encoding/json"
	"fmt"

	"cnb.cool/mliev/push/message-push/app/model"
)

// PrepareContent is shared by delivery and the read-only legacy preview. Its
// output is also the input to the sender, keeping the audit and send semantics equal.
func PrepareContent(renderer Renderer, rawParams string, binding *model.ChannelTemplateBinding) model.MessageContent {
	result := model.MessageContent{
		OriginalParamsRaw: rawParams,
		ContentType:       "text",
		ParamMapping:      []model.ParamMappingItem{},
		MappingDetails:    []model.ResolvedParamMapping{},
	}
	paramsValid := true
	if rawParams != "" {
		if err := json.Unmarshal([]byte(rawParams), &result.OriginalParams); err != nil {
			result.OriginalParams = nil
			result.UnavailableReason = "模板参数不是有效的字符串对象"
			paramsValid = false
		}
	}
	if binding == nil || binding.ProviderTemplate == nil {
		result.UnavailableReason = "供应商模板不存在，无法生成发送内容"
		return result
	}
	t := binding.ProviderTemplate
	result.BindingID = binding.ID
	result.ProviderTemplateID = t.ID
	result.TemplateCode = t.TemplateCode
	result.TemplateName = t.TemplateName
	result.TemplateContent = t.TemplateContent
	if t.ContentType != "" {
		result.ContentType = t.ContentType
	}

	mapping, mappingErr := binding.GetParamMapping()
	if mappingErr != nil {
		result.UnavailableReason = "参数映射配置无效"
	} else {
		result.ParamMapping = append(result.ParamMapping, mapping...)
		if len(mapping) > 0 {
			// An empty raw parameter string historically skips mapping, even for
			// fixed values. Preserve that behavior for already queued tasks.
			if paramsValid && rawParams != "" {
				result.MappedParams = renderer.MapParams(result.OriginalParams, mapping)
			}
			for _, item := range mapping {
				row := model.ResolvedParamMapping{
					ProviderVar: item.ProviderVar, Type: string(item.Type),
					SystemVar: item.SystemVar, FixedValue: item.Value,
				}
				if item.Type != model.ParamMappingTypeFixed {
					_, exists := result.OriginalParams[item.SystemVar]
					row.Missing = !exists
				}
				result.MappingDetails = append(result.MappingDetails, row)
			}
		} else {
			variables, err := t.GetVariables()
			if err != nil {
				result.UnavailableReason = "供应商模板变量配置无效"
			} else {
				if paramsValid && rawParams != "" {
					result.MappedParams = SameNameParams(result.OriginalParams, variables)
				}
				for _, variable := range variables {
					_, exists := result.OriginalParams[variable]
					result.MappingDetails = append(result.MappingDetails, model.ResolvedParamMapping{
						ProviderVar: variable, Type: "same_name", SystemVar: variable, Missing: !exists,
					})
				}
			}
		}
	}
	for i := range result.MappingDetails {
		row := &result.MappingDetails[i]
		if value, exists := result.MappedParams[row.ProviderVar]; exists {
			row.Value = &value
		}
		if row.Missing {
			result.Warnings = append(result.Warnings, fmt.Sprintf("缺少来源参数：%s", row.SystemVar))
		}
	}
	if t.TemplateContent == "" {
		result.UnavailableReason = "供应商模板未保存正文，无法生成发送内容"
		return result
	}
	content, err := renderer.RenderSimple(t.TemplateContent, result.MappedParams)
	if err != nil {
		result.UnavailableReason = "供应商模板渲染失败"
		return result
	}
	result.Content = content
	return result
}

// SameNameParams only forwards variables declared by the provider template.
func SameNameParams(params map[string]string, variables []string) map[string]string {
	result := make(map[string]string, len(variables))
	for _, variable := range variables {
		if value, exists := params[variable]; exists {
			result[variable] = value
		}
	}
	return result
}
