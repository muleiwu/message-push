package domain

import (
	"fmt"
	"strings"

	"cnb.cool/mliev/push/message-push/app/model"
)

// TemplateCodec owns native syntax. Account configuration may select a provider's send mode.
type TemplateCodec interface {
	Version() string
	Parse(string, *model.ProviderAccount) (*ParsedTemplate, error)
}

type TemplateToken struct {
	Start, End int
	Key        string
}

// ParsedTemplate is derived from native content and is never persisted as a second template.
type ParsedTemplate struct {
	Content         string          `json:"content"`
	Variables       []string        `json:"variables"`
	NativeVariables []string        `json:"native_variables"`
	SystemContent   string          `json:"system_content"`
	Tokens          []TemplateToken `json:"-"`
	SendOrder       []string        `json:"-"`
}

type TemplateParameters struct {
	Named   map[string]string
	Ordered []string
}

func (p *ParsedTemplate) Bind(values map[string]string) (*TemplateParameters, error) {
	result := &TemplateParameters{Named: map[string]string{}, Ordered: []string{}}
	for _, key := range p.Variables {
		value, ok := values[key]
		if !ok {
			return nil, fmt.Errorf("缺少模板变量 %s", key)
		}
		result.Named[key] = value
	}
	for _, key := range p.SendOrder {
		result.Ordered = append(result.Ordered, result.Named[key])
	}
	return result, nil
}

func (p *ParsedTemplate) Render(values map[string]string) (string, error) {
	if _, err := p.Bind(values); err != nil {
		return "", err
	}
	var out strings.Builder
	start := 0
	for _, token := range p.Tokens {
		out.WriteString(p.Content[start:token.Start])
		out.WriteString(values[token.Key])
		start = token.End
	}
	out.WriteString(p.Content[start:])
	return out.String(), nil
}

func IsSMSTemplate(t *model.ProviderTemplate) bool {
	if t == nil || t.ProviderAccount == nil {
		return false
	}
	if t.ProviderAccount.ProviderType == "sms" {
		return true
	}
	meta, err := GetByCode(t.ProviderAccount.ProviderCode)
	return err == nil && meta.Type == "sms"
}

func ParseProviderTemplate(t *model.ProviderTemplate) (*ParsedTemplate, error) {
	if t == nil || t.ProviderAccount == nil {
		return nil, fmt.Errorf("供应商账号不存在，无法解析模板")
	}
	return ParseAccountTemplate(t.ProviderAccount, t.TemplateContent)
}

func ParseAccountTemplate(account *model.ProviderAccount, content string) (*ParsedTemplate, error) {
	if account == nil {
		return nil, fmt.Errorf("供应商账号不存在")
	}
	meta, err := GetByCode(account.ProviderCode)
	if err != nil {
		return nil, err
	}
	if meta.TemplateCodec == nil {
		return nil, fmt.Errorf("供应商未声明原生模板解析能力")
	}
	return meta.TemplateCodec.Parse(content, account)
}

func ProviderTemplateVariables(t *model.ProviderTemplate) ([]string, error) {
	if IsSMSTemplate(t) {
		parsed, err := ParseProviderTemplate(t)
		if err != nil {
			return nil, err
		}
		return parsed.Variables, nil
	}
	if t == nil {
		return nil, fmt.Errorf("供应商模板不存在")
	}
	return t.GetVariables()
}

func MappingConfirmed(binding *model.ChannelTemplateBinding) bool {
	return binding != nil && binding.ProviderTemplate != nil &&
		(!IsSMSTemplate(binding.ProviderTemplate) || (binding.ProviderTemplate.ContentVersion > 0 && binding.MappedContentVersion == binding.ProviderTemplate.ContentVersion))
}
