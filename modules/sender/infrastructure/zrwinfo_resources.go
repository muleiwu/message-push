package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

const zrwinfoResourceBaseURL = "https://api.infosoo.com"

func zrwinfoResourceDefinitions() map[domain.ResourceKind]*domain.ResourceDefinition {
	return newZrwinfoResourceDefinitions(&http.Client{Timeout: time.Duration(domain.DefaultTimeout) * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, zrwinfoResourceBaseURL)
}

func newZrwinfoResourceDefinitions(client *http.Client, baseURL string) map[domain.ResourceKind]*domain.ResourceDefinition {
	result := map[domain.ResourceKind]*domain.ResourceDefinition{}
	for _, kind := range []domain.ResourceKind{domain.ResourceTemplates, domain.ResourceSignatures} {
		isTemplate := kind == domain.ResourceTemplates
		name, id, list := "Sign", "signId", "signlist"
		fields := []domain.ResourceField{{Name: "content", Label: "供应商签名", Type: "text", Required: true}, {Name: "description", Label: "用途说明", Type: "textarea"}}
		response := map[string][]string{"id": {"signId", "id"}, "content": {"sign"}, "audit_status": {"applyStatus"}, "audit_reply": {"applyReply"}, "description": {"description"}}
		request := map[string]string{"content": "sign", "description": "description"}
		if isTemplate {
			name, id, list = "Template", "templateId", "templatelist"
			fields = []domain.ResourceField{{Name: "name", Label: "供应商模板名称", Type: "text", Required: true}, {Name: "content", Label: "模板内容", Type: "template", Required: true}, {Name: "category", Label: "短信类别", Type: "select", Required: true, Options: []domain.FieldOption{{Value: "1", Label: "验证码"}, {Value: "2", Label: "通知订单"}, {Value: "3", Label: "营销"}}}, {Name: "description", Label: "用途说明", Type: "textarea", Required: true}}
			response = map[string][]string{"id": {"templateId", "id"}, "name": {"templateName"}, "content": {"template"}, "category": {"categoryId"}, "audit_status": {"applyStatus"}, "audit_reply": {"applyReply"}, "description": {"description"}}
			request = map[string]string{"name": "templateName", "content": "template", "category": "categoryId", "description": "description"}
		}
		definition := &domain.ResourceDefinition{Operations: map[domain.ResourceAction]*domain.ResourceOperation{}}
		if !isTemplate {
			definition.MatchAliases = func(content string) []string {
				plain := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(content), "【"), "】")
				return []string{content, plain, "【" + plain + "】"}
			}
		}
		for _, action := range []domain.ResourceAction{domain.ResourceQuery, domain.ResourceCreate, domain.ResourceUpdate, domain.ResourceDelete} {
			protocol := domain.ResourceProtocol{SuccessValue: "0", DataField: "data", ResponseFields: response, RequestFields: map[string]string{}, AuditStatuses: map[string]int8{"1": 1, "2": 2, "3": 3}}
			op := &domain.ResourceOperation{Fields: []domain.ResourceField{}}
			switch action {
			case domain.ResourceQuery:
				protocol.Path = "/query/" + list
				protocol.SuccessField = "code"
				if isTemplate {
					protocol.DetailPath = "/query/getTemplate"
					protocol.DetailIDField = "templateId"
				}
			case domain.ResourceCreate, domain.ResourceUpdate:
				protocol.Path = "/open/api/add" + name
				protocol.SuccessField = "ret"
				for key, value := range request {
					protocol.RequestFields[key] = value
				}
				for _, field := range fields {
					if action == domain.ResourceUpdate && field.Name == "category" {
						continue
					}
					op.Fields = append(op.Fields, field)
				}
				if action == domain.ResourceUpdate {
					protocol.Path = "/open/api/modify" + name
					protocol.RequestFields["id"] = "id"
					delete(protocol.RequestFields, "category")
				}
			case domain.ResourceDelete:
				protocol.Path = "/open/query/del" + name
				protocol.SuccessField = "code"
				protocol.RequestFields["id"] = id
			}
			op.Protocol = protocol
			currentAction := action
			op.Handler = func(ctx context.Context, account *model.ProviderAccount, input domain.ResourceInput) ([]domain.RemoteResource, error) {
				return executeZrwinfoResource(ctx, client, baseURL, protocol, currentAction, account, input)
			}
			definition.Operations[action] = op
		}
		result[kind] = definition
	}
	return result
}

func executeZrwinfoResource(ctx context.Context, client *http.Client, baseURL string, p domain.ResourceProtocol, action domain.ResourceAction, account *model.ProviderAccount, input domain.ResourceInput) ([]domain.RemoteResource, error) {
	config, err := account.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("供应商账号配置无效")
	}
	key, _ := config["accesskey"].(string)
	secret, _ := config["secret"].(string)
	if key == "" || secret == "" {
		return nil, fmt.Errorf("请先配置 AccessKey 和 Secret")
	}
	values := map[string]string{"id": input.ID, "name": input.Name, "content": input.Content, "category": input.Category, "description": input.Description}
	form := url.Values{"accesskey": {key}, "secret": {secret}}
	for source, target := range p.RequestFields {
		form.Set(target, values[source])
	}
	path := p.Path
	method := http.MethodPost
	if action == domain.ResourceQuery && input.ID != "" && p.DetailPath != "" {
		path = p.DetailPath
		form.Set(p.DetailIDField, input.ID)
		method = http.MethodGet
	}
	endpoint := baseURL + path
	var body io.Reader = strings.NewReader(form.Encode())
	if method == http.MethodGet {
		endpoint += "?" + form.Encode()
		body = nil
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("无法构造供应商请求")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	uncertain := action != domain.ResourceQuery
	failure := func(code, message string, unknown bool) error {
		message = strings.ReplaceAll(strings.ReplaceAll(message, key, "[redacted]"), secret, "[redacted]")
		return &domain.RemoteResourceError{Code: code, Message: message, Uncertain: unknown}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, failure("TRANSPORT_ERROR", "供应商请求未完成，请查询资源确认执行结果", uncertain)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, failure("INVALID_RESPONSE", "读取供应商响应失败", uncertain)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, failure("HTTP_ERROR", fmt.Sprintf("供应商返回 HTTP %d", resp.StatusCode), uncertain && resp.StatusCode >= 500)
	}
	var envelope map[string]json.RawMessage
	if err = json.Unmarshal(raw, &envelope); err != nil {
		return nil, failure("INVALID_RESPONSE", "供应商返回无效 JSON", uncertain)
	}
	code, err := resourceScalar(envelope[p.SuccessField])
	if err != nil || code == "" {
		return nil, failure("INVALID_RESPONSE", "供应商响应缺少成功码", uncertain)
	}
	if code != p.SuccessValue {
		msg, _ := resourceScalar(envelope["msg"])
		if msg == "" {
			msg = "供应商拒绝操作"
		}
		return nil, failure(code, msg, false)
	}
	if action == domain.ResourceDelete {
		return []domain.RemoteResource{{ResourceInput: domain.ResourceInput{ID: input.ID}}}, nil
	}
	data := bytes.TrimSpace(envelope[p.DataField])
	if len(data) > 0 && data[0] == '"' {
		var nested string
		if json.Unmarshal(data, &nested) == nil {
			data = []byte(nested)
		}
	}
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		return nil, failure("INVALID_RESPONSE", "供应商响应缺少资源数据", uncertain)
	}
	var records []map[string]json.RawMessage
	if data[0] == '[' {
		err = json.Unmarshal(data, &records)
	} else {
		var record map[string]json.RawMessage
		err = json.Unmarshal(data, &record)
		records = []map[string]json.RawMessage{record}
	}
	if err != nil {
		return nil, failure("INVALID_RESPONSE", "供应商资源数据格式错误", uncertain)
	}
	out := make([]domain.RemoteResource, 0, len(records))
	seen := map[string]bool{}
	for _, record := range records {
		mapped := map[string]string{}
		for name, aliases := range p.ResponseFields {
			for _, alias := range aliases {
				if value, exists := record[alias]; exists && !bytes.Equal(value, []byte("null")) {
					text, e := resourceScalar(value)
					if e != nil {
						return nil, failure("INVALID_RESPONSE", "供应商资源字段类型错误: "+alias, uncertain)
					}
					if previous, ok := mapped[name]; ok && previous != text {
						return nil, failure("INVALID_RESPONSE", "供应商资源字段冲突: "+name, uncertain)
					}
					mapped[name] = text
				}
			}
		}
		if mapped["id"] == "" || mapped["id"] == "0" {
			return nil, failure("INVALID_RESPONSE", "供应商响应缺少资源 ID", uncertain)
		}
		if seen[mapped["id"]] {
			return nil, failure("INVALID_RESPONSE", "供应商返回重复资源 ID", uncertain)
		}
		seen[mapped["id"]] = true
		if action == domain.ResourceQuery && input.ID != "" && mapped["id"] != input.ID {
			continue
		}
		if action == domain.ResourceQuery && strings.TrimSpace(mapped["content"]) == "" {
			return nil, failure("INVALID_RESPONSE", "供应商资源缺少内容", false)
		}
		if action == domain.ResourceUpdate && mapped["id"] != input.ID {
			return nil, failure("INVALID_RESPONSE", "供应商返回的资源 ID 与请求不一致", true)
		}
		status := p.AuditStatuses[mapped["audit_status"]]
		out = append(out, domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: mapped["id"], Name: mapped["name"], Content: mapped["content"], Category: mapped["category"], Description: mapped["description"]}, AuditStatus: int8(status), AuditReply: mapped["audit_reply"]})
	}
	if input.ID != "" && len(out) == 0 {
		return nil, failure("NOT_FOUND", "供应商资源不存在", false)
	}
	return out, nil
}

func resourceScalar(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "", nil
	}
	var value any
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&value); err != nil {
		return "", err
	}
	switch v := value.(type) {
	case string:
		return v, nil
	case json.Number:
		return v.String(), nil
	default:
		return "", fmt.Errorf("not a scalar")
	}
}
