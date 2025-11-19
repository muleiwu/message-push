package helper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
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

// Render 渲染模板
func (h *TemplateHelper) Render(templateCode string, params map[string]interface{}) (string, error) {
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

// getTemplateContent 获取模板内容
func (h *TemplateHelper) getTemplateContent(templateCode string) (string, error) {
	// 预定义模板
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
func (h *TemplateHelper) RenderJSON(params map[string]interface{}) (string, error) {
	data, err := json.Marshal(params)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
