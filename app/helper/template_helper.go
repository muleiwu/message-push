package helper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"cnb.cool/mliev/push/message-push/app/model"
)

// TemplateHelper 模板助手
type TemplateHelper struct {
	templates map[string]*template.Template
}

// NewTemplateHelper 创建模板助手
func NewTemplateHelper() *TemplateHelper {
	return &TemplateHelper{
		templates: make(map[string]*template.Template),
	}
}

// Render 渲染模板（支持旧的{{.variable}}和新的{variable}格式）
func (h *TemplateHelper) Render(templateCode string, params map[string]string) (string, error) {
	templateContent, err := h.getTemplateContent(templateCode)
	if err != nil {
		return "", fmt.Errorf("template not found: %s", templateCode)
	}

	tmpl, err := template.New(templateCode).Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// RenderSimple 渲染简单模板（使用{variable}占位符格式）
func (h *TemplateHelper) RenderSimple(templateContent string, params map[string]string) (string, error) {
	result := templateContent

	// 使用正则表达式匹配 {variable} 格式
	re := regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

	result = re.ReplaceAllStringFunc(result, func(match string) string {
		// 提取变量名（去掉大括号）
		varName := strings.Trim(match, "{}")

		// 从参数中获取值
		if value, exists := params[varName]; exists {
			return value
		}

		// 如果参数不存在，保持原样
		return match
	})

	return result, nil
}

// MapParams 根据参数映射转换参数，返回有序的参数值数组
// params: 用户传入的系统变量参数 map[string]string
// mapping: 参数映射配置 []ParamMappingItem
// 返回: 有序的参数值数组（按 mapping 顺序），用于腾讯云等需要有序参数的服务商
func (h *TemplateHelper) MapParams(params map[string]string, mapping []model.ParamMappingItem) []string {
	if len(mapping) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(mapping))

	for _, item := range mapping {
		var value string
		switch item.Type {
		case model.ParamMappingTypeFixed:
			// 固定值：直接使用配置的 Value
			value = item.Value
		case model.ParamMappingTypeMapping:
			// 映射：从用户参数中获取系统变量的值
			if v, exists := params[item.SystemVar]; exists {
				value = v
			}
		default:
			// 默认按映射处理
			if v, exists := params[item.SystemVar]; exists {
				value = v
			}
		}
		result = append(result, value)
	}

	return result
}

// MapParamsToMap 根据参数映射转换参数，返回 map 格式
// params: 用户传入的系统变量参数 map[string]string
// mapping: 参数映射配置 []ParamMappingItem
// 返回: 供应商变量名到值的映射，用于阿里云等需要 map 参数的服务商
func (h *TemplateHelper) MapParamsToMap(params map[string]string, mapping []model.ParamMappingItem) map[string]string {
	if len(mapping) == 0 {
		return params
	}

	result := make(map[string]string)

	for _, item := range mapping {
		var value string
		switch item.Type {
		case model.ParamMappingTypeFixed:
			// 固定值：直接使用配置的 Value
			value = item.Value
		case model.ParamMappingTypeMapping:
			// 映射：从用户参数中获取系统变量的值
			if v, exists := params[item.SystemVar]; exists {
				value = v
			}
		default:
			// 默认按映射处理
			if v, exists := params[item.SystemVar]; exists {
				value = v
			}
		}
		result[item.ProviderVar] = value
	}

	return result
}

// getTemplateContent 获取模板内容（保留向后兼容的预定义模板）
func (h *TemplateHelper) getTemplateContent(templateCode string) (string, error) {
	// 预定义模板（向后兼容）
	templates := map[string]string{
		"verify_code":   "您的验证码是：{{.code}}，有效期{{.expire}}分钟。",
		"marketing_sms": "尊敬的{{.name}}，我们的新产品已上线，欢迎体验！",
		"order_notify":  "您的订单{{.order_id}}已发货，预计{{.delivery_date}}送达。",
	}

	content, exists := templates[templateCode]
	if !exists {
		return "", fmt.Errorf("template not found")
	}

	return content, nil
}

// RenderJSON 渲染为JSON格式
func (h *TemplateHelper) RenderJSON(params map[string]string) (string, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
