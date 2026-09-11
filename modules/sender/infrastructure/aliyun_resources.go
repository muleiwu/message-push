package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func aliyunResourceDefinitions(factory aliyunClientFactory) map[domain.ResourceKind]*domain.ResourceDefinition {
	definitions := map[domain.ResourceKind]*domain.ResourceDefinition{}
	for _, kind := range []domain.ResourceKind{domain.ResourceTemplates, domain.ResourceSignatures} {
		resource, idField, list := "Sign", "SignName", "SmsSignList"
		fields := []domain.ResourceField{
			{Name: "content", Label: "短信签名", Type: "text", Required: true, Help: "填写 2～12 个中文、英文字母或数字，不含括号；编辑时不能修改签名名称。"},
			{Name: "provider_fields.qualification_id", Label: "资质 ID", Type: "text", Required: true, Help: "填写阿里云已审核通过的资质 ID，自用或他用须与资质一致。"},
			{Name: "provider_fields.sign_source", Label: "签名来源", Type: "select", Required: true, Options: []domain.FieldOption{{Value: "0", Label: "企事业单位"}, {Value: "5", Label: "商标"}}},
			{Name: "provider_fields.sign_type", Label: "签名类型", Type: "select", Options: []domain.FieldOption{{Value: "1", Label: "通用"}, {Value: "0", Label: "验证码"}}, Help: "新增时默认为通用；编辑时请明确选择，详情接口不返回此字段。"},
			{Name: "provider_fields.third_party", Label: "签名用途", Type: "select", Options: []domain.FieldOption{{Value: "false", Label: "自用"}, {Value: "true", Label: "他用"}}, Help: "默认为自用。"},
			{Name: "provider_fields.authorization_letter_id", Label: "授权委托书 ID", Type: "text", RequiredWhen: map[string]string{"provider_fields.third_party": "true"}},
			{Name: "provider_fields.trademark_id", Label: "商标 ID", Type: "text", RequiredWhen: map[string]string{"provider_fields.sign_source": "5"}},
			{Name: "description", Label: "申请说明", Type: "textarea"},
			{Name: "provider_fields.more_data", Label: "补充材料 OSS 文件引用", Type: "textarea", Sensitive: true, Help: "填写已上传文件的 JSON 数组，例如 [\"账号ID/材料.png\"]；仅用于本次提交，不支持临时下载链接。"},
		}
		wire := map[string]string{"content": "SignName", "description": "Remark", "provider_fields.qualification_id": "QualificationId", "provider_fields.sign_source": "SignSource", "provider_fields.sign_type": "SignType", "provider_fields.third_party": "ThirdParty", "provider_fields.authorization_letter_id": "AuthorizationLetterId", "provider_fields.trademark_id": "TrademarkId", "provider_fields.more_data": "MoreData"}
		if kind == domain.ResourceTemplates {
			resource, idField, list = "Template", "TemplateCode", "SmsTemplateList"
			fields = []domain.ResourceField{
				{Name: "name", Label: "模板名称", Type: "text", Required: true, Help: "不超过 30 个字符。"},
				{Name: "content", Label: "阿里云模板原文", Type: "template", Required: true, Help: "不超过 500 个字符，使用 ${code} 等命名变量；业务参数在通道中映射。"},
				{Name: "category", Label: "短信类型", Type: "select", Required: true, Options: []domain.FieldOption{{Value: "0", Label: "验证码"}, {Value: "1", Label: "通知"}, {Value: "2", Label: "推广"}}},
				{Name: "provider_fields.related_sign_name", Label: "关联签名", Type: "text", Required: true, Help: "填写本账号已审核通过的签名名称；仅用于模板审核。"},
				{Name: "provider_fields.template_rule", Label: "变量规则 JSON", Type: "textarea", Help: "有变量时必填，必须覆盖全部变量，例如 {\"code\":\"characterWithNumber\"}；规则取值遵循阿里云规范。"},
				{Name: "provider_fields.apply_scene_content", Label: "业务场景", Type: "textarea", Required: true},
				{Name: "description", Label: "申请说明", Type: "textarea"},
				{Name: "provider_fields.more_data", Label: "补充材料 OSS 文件引用", Type: "textarea", Sensitive: true, RequiredWhen: map[string]string{"category": "2"}, Help: "填写已上传文件的 JSON 数组，例如 [\"账号ID/材料.png\"]；推广短信须提供用户授权证明。仅用于本次提交。"},
				{Name: "provider_fields.traffic_driving", Label: "引流信息 JSON", Type: "textarea", Sensitive: true, Help: "含链接或电话号码时按阿里云规范填写 JSON 数组；仅用于本次提交。"},
			}
			wire = map[string]string{"name": "TemplateName", "content": "TemplateContent", "category": "TemplateType", "description": "Remark", "provider_fields.related_sign_name": "RelatedSignName", "provider_fields.template_rule": "TemplateRule", "provider_fields.apply_scene_content": "ApplySceneContent", "provider_fields.more_data": "MoreData", "provider_fields.traffic_driving": "TrafficDriving"}
		}
		definition := &domain.ResourceDefinition{Operations: map[domain.ResourceAction]*domain.ResourceOperation{}, AuditOrderID: aliyunOrderID}
		for _, action := range []domain.ResourceAction{domain.ResourceQuery, domain.ResourceCreate, domain.ResourceUpdate, domain.ResourceDelete} {
			verb := map[domain.ResourceAction]string{domain.ResourceQuery: "Query", domain.ResourceCreate: "Create", domain.ResourceUpdate: "Update", domain.ResourceDelete: "Delete"}[action]
			op := &domain.ResourceOperation{Protocol: domain.ResourceProtocol{Path: verb + "Sms" + resource, SuccessField: "Code", SuccessValue: "OK", DataField: "$", RequestFields: map[string]string{"id": idField}, ResponseFields: map[string][]string{"id": {idField}}}}
			if action == domain.ResourceQuery {
				op.Protocol.Path += "List"
				op.Protocol.DataField, op.Protocol.DetailPath, op.Protocol.DetailIDField = list, "GetSms"+resource, idField
			}
			if action == domain.ResourceCreate || action == domain.ResourceUpdate {
				op.Fields = append([]domain.ResourceField{}, fields...)
				for key, value := range wire {
					op.Protocol.RequestFields[key] = value
				}
				if kind == domain.ResourceSignatures && action == domain.ResourceUpdate {
					for i := range op.Fields {
						if op.Fields[i].Name == "content" {
							op.Fields[i].ReadOnly = true
						}
						if op.Fields[i].Name == "provider_fields.sign_type" {
							op.Fields[i].Required = true
						}
					}
				}
			}
			if action == domain.ResourceUpdate || action == domain.ResourceDelete {
				op.SuspendBeforeWrite = true
				op.ValidateCurrent = func(remote domain.RemoteResource) error { return validateAliyunCurrent(kind, action, remote) }
			}
			op.ValidateInput = func(input domain.ResourceInput) error { return validateAliyunResource(kind, action, input) }
			op.Handler = func(ctx context.Context, account *model.ProviderAccount, input domain.ResourceInput) ([]domain.RemoteResource, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				client, err := factory(account)
				if err != nil {
					return nil, err
				}
				if action == domain.ResourceQuery {
					return queryAliyunResources(ctx, client, kind, input.ID)
				}
				if action != domain.ResourceCreate {
					rows, err := queryAliyunResources(ctx, client, kind, input.ID)
					if err != nil {
						return nil, err
					}
					if len(rows) != 1 {
						return nil, aliyunInvalidResponse("", false)
					}
					if err := validateAliyunCurrent(kind, action, rows[0]); err != nil {
						return nil, err
					}
				}
				return mutateAliyunResource(ctx, client, kind, action, input)
			}
			definition.Operations[action] = op
		}
		definitions[kind] = definition
	}
	return definitions
}

func validateAliyunCurrent(kind domain.ResourceKind, action domain.ResourceAction, remote domain.RemoteResource) error {
	if kind == domain.ResourceTemplates && remote.Category != "0" && remote.Category != "1" && remote.Category != "2" {
		return fmt.Errorf("仅支持管理国内短信模板")
	}
	if action == domain.ResourceUpdate {
		if kind == domain.ResourceTemplates && remote.AuditStatus != 3 {
			return fmt.Errorf("仅审核未通过的模板可以修改；已通过的模板请重新申请")
		}
		if kind == domain.ResourceSignatures && remote.AuditStatus != 2 && remote.AuditStatus != 3 {
			return fmt.Errorf("仅审核通过或未通过的签名可以修改")
		}
	}
	if action == domain.ResourceDelete && remote.AuditStatus != 2 && remote.AuditStatus != 3 {
		metadata, _ := remote.ProviderMetadata["aliyun"].(map[string]any)
		raw, _ := metadata["audit_status"].(string)
		if raw != "10" && raw != "AUDIT_STATE_CANCEL" && raw != "AUDIT_SATE_CANCEL" {
			return fmt.Errorf("审核中或状态未知的资源不能删除")
		}
	}
	return nil
}

func aliyunPositiveID(value, label string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != value {
		return 0, fmt.Errorf("%s须为有效的正整数", label)
	}
	return id, nil
}

func validateAliyunResource(kind domain.ResourceKind, action domain.ResourceAction, input domain.ResourceInput) error {
	if input.ID != "" && (strings.TrimSpace(input.ID) != input.ID || utf8.RuneCountInString(input.ID) > 100) {
		return fmt.Errorf("资源标识无效")
	}
	if action == domain.ResourceQuery || action == domain.ResourceDelete {
		return nil
	}
	allowed := map[string]bool{"more_data": true}
	if kind == domain.ResourceTemplates {
		for _, key := range []string{"related_sign_name", "template_rule", "apply_scene_content", "traffic_driving"} {
			allowed[key] = true
		}
		if utf8.RuneCountInString(input.Name) > 30 || utf8.RuneCountInString(input.Content) > 500 {
			return fmt.Errorf("模板名称不得超过 30 字符，正文不得超过 500 字符")
		}
		parsed, err := (NativeTemplateCodec{ID: "aliyun-native-v1", Prefix: "$", AllowNamed: true}).Parse(input.Content, nil)
		if err != nil {
			return err
		}
		rule := input.ProviderFields["template_rule"]
		var rules map[string]string
		if rule != "" && (len(rule) > 65536 || json.Unmarshal([]byte(rule), &rules) != nil || rules == nil) {
			return fmt.Errorf("变量规则须为 JSON 对象，值为阿里云变量规则名称")
		}
		if len(rules) != len(parsed.Variables) {
			return fmt.Errorf("变量规则必须完整覆盖模板中的变量，且不得包含多余变量")
		}
		for _, name := range parsed.Variables {
			if strings.TrimSpace(rules[name]) == "" {
				return fmt.Errorf("请填写变量 %s 的阿里云规则", name)
			}
		}
		if value := input.ProviderFields["traffic_driving"]; value != "" {
			var entries []map[string]any
			if len(value) > 65536 || json.Unmarshal([]byte(value), &entries) != nil || entries == nil {
				return fmt.Errorf("引流信息须为 JSON 对象数组")
			}
			for _, entry := range entries {
				if entry == nil || entry["trafficDrivingType"] == nil || entry["trafficDrivingContent"] == nil {
					return fmt.Errorf("引流信息需提供 trafficDrivingType 和 trafficDrivingContent")
				}
			}
		}
	} else {
		for _, key := range []string{"qualification_id", "sign_source", "sign_type", "third_party", "authorization_letter_id", "trademark_id"} {
			allowed[key] = true
		}
		if action == domain.ResourceUpdate && input.Content != input.ID {
			return fmt.Errorf("阿里云签名名称不能通过修改接口更改")
		}
		n := utf8.RuneCountInString(input.Content)
		if n < 2 || n > 12 || utf8.RuneCountInString(input.Description) > 200 {
			return fmt.Errorf("签名须为 2～12 个字符，申请说明不得超过 200 字符")
		}
		onlyNumbers := true
		for _, r := range input.Content {
			if !(unicode.Is(unicode.Han, r) || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
				return fmt.Errorf("签名仅支持中文、英文字母和数字，不包含括号或空格")
			}
			if r < '0' || r > '9' {
				onlyNumbers = false
			}
		}
		if onlyNumbers {
			return fmt.Errorf("签名不能为纯数字")
		}
		for key, label := range map[string]string{"qualification_id": "资质 ID", "authorization_letter_id": "授权委托书 ID", "trademark_id": "商标 ID"} {
			if value := input.ProviderFields[key]; value != "" {
				if _, err := aliyunPositiveID(value, label); err != nil {
					return err
				}
			}
		}
	}
	for key := range input.ProviderFields {
		if !allowed[key] {
			return fmt.Errorf("不支持的阿里云申请字段：%s", key)
		}
	}
	_, err := aliyunMaterialReferences(input.ProviderFields["more_data"])
	return err
}

func aliyunMaterialReferences(value string) ([]*string, error) {
	if value == "" {
		return nil, nil
	}
	var refs []string
	if len(value) > 65536 || json.Unmarshal([]byte(value), &refs) != nil || len(refs) == 0 {
		return nil, fmt.Errorf("补充材料须为非空 OSS 文件引用 JSON 数组")
	}
	result := make([]*string, 0, len(refs))
	for _, ref := range refs {
		if strings.TrimSpace(ref) == "" || strings.ContainsAny(ref, "?\r\n") || strings.Contains(ref, "://") || !strings.Contains(ref, "/") {
			return nil, fmt.Errorf("补充材料请填写 OSS 文件引用，不使用下载链接")
		}
		result = append(result, &ref)
	}
	return result, nil
}
