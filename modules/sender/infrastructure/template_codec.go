package infrastructure

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

var logicalPlaceholder = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)
var positionalPlaceholder = regexp.MustCompile(`\{([1-9][0-9]*)\}`)

type PositionalTemplateCodec struct{}

func (PositionalTemplateCodec) Version() string { return "positional-v1" }

func validatePlaceholders(content string, pattern *regexp.Regexp) error {
	remainder := pattern.ReplaceAllString(content, "")
	if strings.ContainsAny(remainder, "{}") || strings.Contains(content, "${") || regexp.MustCompile(`#[^#\s]+#`).MatchString(content) {
		return fmt.Errorf("无法识别模板占位符，请使用声明的变量格式")
	}
	return nil
}

func (c PositionalTemplateCodec) Compile(content string) (*domain.CompiledTemplate, error) {
	if err := validatePlaceholders(content, logicalPlaceholder); err != nil {
		return nil, err
	}
	out := &domain.CompiledTemplate{Content: content, Version: c.Version(), Variables: []string{}, Slots: []domain.VariableSlot{}}
	seen := map[string]bool{}
	out.NativeContent = logicalPlaceholder.ReplaceAllStringFunc(content, func(token string) string {
		name := token[1 : len(token)-1]
		position := strconv.Itoa(len(out.Slots) + 1)
		out.Slots = append(out.Slots, domain.VariableSlot{Native: position, Name: name})
		if !seen[name] {
			out.Variables = append(out.Variables, name)
			seen[name] = true
		}
		return "{" + position + "}"
	})
	return out, nil
}

func (c PositionalTemplateCodec) Decode(content string, previous []domain.VariableSlot) (*domain.CompiledTemplate, error) {
	if err := validatePlaceholders(content, positionalPlaceholder); err != nil {
		return nil, err
	}
	positions := map[int]bool{}
	for _, m := range positionalPlaceholder.FindAllStringSubmatch(content, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n > 1000 {
			return nil, fmt.Errorf("模板变量位置超出范围")
		}
		positions[n] = true
	}
	old := map[string]string{}
	for _, slot := range previous {
		old[slot.Native] = slot.Name
	}
	out := &domain.CompiledTemplate{NativeContent: content, Version: c.Version(), Variables: []string{}, Slots: []domain.VariableSlot{}}
	seen := map[string]bool{}
	for i := 1; i <= len(positions); i++ {
		if !positions[i] {
			return nil, fmt.Errorf("模板变量位置不连续，缺少 {%d}", i)
		}
		key := strconv.Itoa(i)
		name := old[key]
		if name == "" {
			name = "var" + key
		}
		if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(name) {
			return nil, fmt.Errorf("invalid stored variable name")
		}
		out.Slots = append(out.Slots, domain.VariableSlot{Native: key, Name: name})
		if !seen[name] {
			out.Variables = append(out.Variables, name)
			seen[name] = true
		}
	}
	out.Content = positionalPlaceholder.ReplaceAllStringFunc(content, func(token string) string {
		n, _ := strconv.Atoi(token[1 : len(token)-1])
		return "{" + out.Slots[n-1].Name + "}"
	})
	return out, nil
}

func (PositionalTemplateCodec) Bind(slots []domain.VariableSlot, values map[string]string) ([]string, error) {
	out := make([]string, len(slots))
	for i, slot := range slots {
		if slot.Native != strconv.Itoa(i+1) {
			return nil, fmt.Errorf("invalid template variable position")
		}
		value, ok := values[slot.Name]
		if !ok {
			return nil, fmt.Errorf("缺少模板变量 %s", slot.Name)
		}
		out[i] = value
	}
	return out, nil
}

// NamedTemplateCodec is reusable by providers with named native placeholders.
// Prefix can be "$" for ${name}; protocol serializers retain ownership of JSON/form encoding.
type NamedTemplateCodec struct{ Prefix string }

func (c NamedTemplateCodec) Version() string { return "named-v1:" + c.Prefix }
func (c NamedTemplateCodec) Compile(content string) (*domain.CompiledTemplate, error) {
	p, err := (PositionalTemplateCodec{}).Compile(content)
	if err != nil {
		return nil, err
	}
	p.Version = c.Version()
	// ReplaceAllString treats '$' as expansion, so use a function for literal prefixes.
	p.NativeContent = logicalPlaceholder.ReplaceAllStringFunc(content, func(t string) string { return c.Prefix + t })
	p.Slots = []domain.VariableSlot{}
	for _, name := range p.Variables {
		p.Slots = append(p.Slots, domain.VariableSlot{Native: name, Name: name})
	}
	return p, nil
}
func (c NamedTemplateCodec) Decode(content string, previous []domain.VariableSlot) (*domain.CompiledTemplate, error) {
	pattern := regexp.MustCompile(regexp.QuoteMeta(c.Prefix) + `\{([a-zA-Z0-9_]+)\}`)
	if strings.ContainsAny(pattern.ReplaceAllString(content, ""), "{}") {
		return nil, fmt.Errorf("无法识别供应商命名变量格式")
	}
	old := map[string]string{}
	for _, slot := range previous {
		old[slot.Native] = slot.Name
	}
	logical := pattern.ReplaceAllStringFunc(content, func(token string) string {
		name := pattern.FindStringSubmatch(token)[1]
		if old[name] != "" {
			name = old[name]
		}
		return "{" + name + "}"
	})
	p, err := c.Compile(logical)
	if err != nil {
		return nil, err
	}
	p.NativeContent = content
	nativeMatches := pattern.FindAllStringSubmatch(content, -1)
	p.Slots = []domain.VariableSlot{}
	seen := map[string]bool{}
	for _, match := range nativeMatches {
		native := match[1]
		if seen[native] {
			continue
		}
		seen[native] = true
		name := old[native]
		if name == "" {
			name = native
		}
		p.Slots = append(p.Slots, domain.VariableSlot{Native: native, Name: name})
	}
	return p, nil
}
func (c NamedTemplateCodec) Bind(slots []domain.VariableSlot, values map[string]string) ([]string, error) {
	result := make([]string, 0, len(slots))
	for _, slot := range slots {
		value, ok := values[slot.Name]
		if !ok {
			return nil, fmt.Errorf("缺少模板变量 %s", slot.Name)
		}
		result = append(result, value)
	}
	return result, nil
}
