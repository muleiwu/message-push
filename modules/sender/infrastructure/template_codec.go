package infrastructure

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

var numericVariableName = regexp.MustCompile(`^[0-9]+$`)
var unsupportedHashPlaceholder = regexp.MustCompile(`#[^#\s]+#`)

// NativeTemplateCodec declares a provider's accepted syntax and parameter order.
// There are no stored aliases or previous mappings involved in parsing.
type NativeTemplateCodec struct {
	ID             string
	Prefix         string
	AllowNamed     bool
	AllowNumeric   bool
	AllowAnonymous bool
	NeteaseModes   bool
}

func (c NativeTemplateCodec) Version() string { return c.ID }

func (c NativeTemplateCodec) Parse(content string, account *model.ProviderAccount) (*domain.ParsedTemplate, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("请填写供应商模板原文")
	}
	pattern := regexp.MustCompile(regexp.QuoteMeta(c.Prefix) + `\{([a-zA-Z0-9_]+)\}`)
	matches := pattern.FindAllStringSubmatchIndex(content, -1)
	anonymous := c.AllowAnonymous && strings.Contains(content, "%s")
	if anonymous && len(matches) > 0 {
		return nil, fmt.Errorf("不能混用匿名与命名占位符")
	}
	if anonymous && c.NeteaseModes && account != nil {
		config, err := account.GetConfig()
		if err != nil {
			return nil, fmt.Errorf("供应商账号配置无效")
		}
		if config["send_type"] == "code" {
			return nil, fmt.Errorf("验证码模式需要命名占位符，不支持 %%s")
		}
	}
	if anonymous {
		matches = regexp.MustCompile(`%s`).FindAllStringSubmatchIndex(content, -1)
	}
	remainder := pattern.ReplaceAllString(content, "")
	if anonymous {
		remainder = strings.ReplaceAll(content, "%s", "")
	}
	if strings.ContainsAny(remainder, "{}") || unsupportedHashPlaceholder.MatchString(remainder) ||
		(!anonymous && strings.Contains(remainder, "%s")) {
		return nil, fmt.Errorf("无法识别模板占位符，请使用供应商声明的原生格式")
	}
	out := &domain.ParsedTemplate{Content: content, Variables: []string{}, NativeVariables: []string{}, SendOrder: []string{}}
	seen, seenTokens := map[string]bool{}, map[string]bool{}
	hasNamed, hasNumeric := false, false
	var system strings.Builder
	end := 0
	for i, match := range matches {
		key := strconv.Itoa(i + 1)
		if !anonymous {
			key = content[match[2]:match[3]]
			numeric := numericVariableName.MatchString(key)
			if numeric {
				hasNumeric = true
				n, err := strconv.Atoi(key)
				if !c.AllowNumeric || err != nil || n < 1 || n > 1000 || strconv.Itoa(n) != key {
					return nil, fmt.Errorf("无效的数字占位符 {%s}", key)
				}
			} else {
				hasNamed = true
				if !c.AllowNamed {
					return nil, fmt.Errorf("此供应商只支持数字占位符")
				}
			}
		}
		if hasNamed && hasNumeric {
			return nil, fmt.Errorf("不能混用数字与命名占位符")
		}
		token := content[match[0]:match[1]]
		if !seen[key] {
			out.Variables = append(out.Variables, key)
			seen[key] = true
		}
		if !seenTokens[token] {
			out.NativeVariables = append(out.NativeVariables, token)
			seenTokens[token] = true
		}
		out.Tokens = append(out.Tokens, domain.TemplateToken{Start: match[0], End: match[1], Key: key})
		out.SendOrder = append(out.SendOrder, key)
		system.WriteString(content[end:match[0]])
		system.WriteString("{" + key + "}")
		end = match[1]
	}
	system.WriteString(content[end:])
	out.SystemContent = system.String()
	if hasNumeric {
		out.SendOrder = []string{}
		for i := 1; i <= len(seen); i++ {
			key := strconv.Itoa(i)
			if !seen[key] {
				return nil, fmt.Errorf("模板变量位置不连续，缺少 {%s}", key)
			}
			out.SendOrder = append(out.SendOrder, key)
		}
	}
	return out, nil
}
