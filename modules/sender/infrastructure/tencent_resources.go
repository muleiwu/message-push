package infrastructure

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strconv"
	"strings"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

func tencentResourceDefinitions(factory tencentClientFactory) map[domain.ResourceKind]*domain.ResourceDefinition {
	result := map[domain.ResourceKind]*domain.ResourceDefinition{}
	for _, kind := range []domain.ResourceKind{domain.ResourceTemplates, domain.ResourceSignatures} {
		fields := []domain.ResourceField{
			{Name: "content", Label: "短信签名", Type: "text", Required: true},
			{Name: "provider_fields.sign_type", Label: "签名类型", Type: "select", Required: true, Options: []domain.FieldOption{{Value: "0", Label: "公司"}, {Value: "4", Label: "商标"}, {Value: "5", Label: "政府或机构"}}},
			{Name: "provider_fields.document_type", Label: "证明类型", Type: "select", Required: true, Options: []domain.FieldOption{{Value: "0", Label: "三证合一"}, {Value: "1", Label: "企业营业执照"}, {Value: "2", Label: "组织机构代码证书"}, {Value: "3", Label: "社会信用代码证书"}, {Value: "7", Label: "商标注册书"}}, Help: "公司选三证合一或营业执照；商标选商标注册书；机构选组织机构或社会信用代码证书。"},
			{Name: "provider_fields.sign_purpose", Label: "签名用途", Type: "select", Required: true, Options: []domain.FieldOption{{Value: "0", Label: "自用"}, {Value: "1", Label: "他用"}}},
			{Name: "provider_fields.qualification_id", Label: "资质 ID", Type: "text", Required: true, Help: "填写腾讯云实名资质管理中已审核通过的国内资质 ID。"},
			{Name: "provider_fields.proof_image", Label: "资质证明图片", Type: "image", Required: true, Sensitive: true, Help: "选择 JPEG 或 PNG 图片；仅用于本次提交，编辑时需重新上传。"},
			{Name: "provider_fields.commission_image", Label: "委托授权证明", Type: "image", Sensitive: true, RequiredWhen: map[string]string{"provider_fields.sign_purpose": "1"}, Help: "他用签名必须提供；仅用于本次提交。"},
			{Name: "description", Label: "申请备注", Type: "textarea"},
		}
		wire := map[string]string{"content": "SignName", "description": "Remark", "provider_fields.sign_type": "SignType", "provider_fields.document_type": "DocumentType", "provider_fields.sign_purpose": "SignPurpose", "provider_fields.qualification_id": "QualificationId", "provider_fields.proof_image": "ProofImage", "provider_fields.commission_image": "CommissionImage"}
		resource, idField, listField := "Sign", "SignId", "DescribeSignListStatusSet"
		if kind == domain.ResourceTemplates {
			resource, idField, listField = "Template", "TemplateId", "DescribeTemplateStatusSet"
			fields = []domain.ResourceField{
				{Name: "name", Label: "模板名称", Type: "text", Required: true},
				{Name: "content", Label: "腾讯云模板原文", Type: "template", Required: true, Help: "使用 {1}、{2} 等连续数字占位符；业务变量在通道中映射。"},
				{Name: "category", Label: "短信类型", Type: "select", Required: true, Options: []domain.FieldOption{{Value: "3", Label: "验证码"}, {Value: "2", Label: "通知"}, {Value: "1", Label: "营销"}}, Help: "腾讯云查询不返回短信类型，编辑时请明确选择。"},
				{Name: "description", Label: "申请备注", Type: "textarea", Required: true},
			}
			wire = map[string]string{"name": "TemplateName", "content": "TemplateContent", "category": "SmsType", "description": "Remark"}
		}
		definition := &domain.ResourceDefinition{Operations: map[domain.ResourceAction]*domain.ResourceOperation{}}
		for _, action := range []domain.ResourceAction{domain.ResourceQuery, domain.ResourceCreate, domain.ResourceUpdate, domain.ResourceDelete} {
			verb := map[domain.ResourceAction]string{domain.ResourceQuery: "Describe", domain.ResourceCreate: "Add", domain.ResourceUpdate: "Modify", domain.ResourceDelete: "Delete"}[action]
			api := verb + "Sms" + resource
			if action == domain.ResourceQuery {
				api += "List"
			}
			op := &domain.ResourceOperation{Protocol: domain.ResourceProtocol{Path: api, ErrorField: "Response.Error", RequestFields: map[string]string{"id": idField}, ResponseFields: map[string][]string{"id": {idField}}, DataField: verb + resource + "Status"}}
			if action == domain.ResourceQuery {
				op.Protocol.DataField = listField
			}
			if action == domain.ResourceCreate || action == domain.ResourceUpdate {
				op.Fields = fields
				for key, value := range wire {
					op.Protocol.RequestFields[key] = value
				}
			}
			currentKind, currentAction := kind, action
			op.ValidateInput = func(input domain.ResourceInput) error {
				return validateTencentResource(currentKind, currentAction, input)
			}
			op.Handler = func(ctx context.Context, account *model.ProviderAccount, input domain.ResourceInput) ([]domain.RemoteResource, error) {
				client, err := factory(account)
				if err != nil {
					return nil, err
				}
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if currentAction == domain.ResourceQuery {
					return queryTencentResources(ctx, client, currentKind, input.ID)
				}
				// Delete APIs have no International argument. Validate the ID using
				// the domestic query before every update/delete, even outside the UI.
				if currentAction != domain.ResourceCreate {
					rows, err := queryTencentResources(ctx, client, currentKind, input.ID)
					if err != nil {
						return nil, err
					}
					if len(rows) != 1 {
						return nil, fmt.Errorf("未找到该账号下的国内短信资源")
					}
				}
				rows, err := mutateTencentResource(ctx, client, currentKind, currentAction, input)
				var remote *domain.RemoteResourceError
				if errors.As(err, &remote) {
					copy := *remote
					config, _ := account.GetConfig()
					for _, value := range []string{input.ProviderFields["proof_image"], input.ProviderFields["commission_image"]} {
						if value != "" {
							copy.Message = strings.ReplaceAll(copy.Message, value, "[redacted]")
						}
					}
					for _, key := range []string{"secret_id", "secret_key"} {
						if value, ok := config[key].(string); ok && value != "" {
							copy.Message = strings.ReplaceAll(copy.Message, value, "[redacted]")
						}
					}
					err = &copy
				}
				return rows, err
			}
			definition.Operations[action] = op
		}
		result[kind] = definition
	}
	return result
}

func tencentPositiveID(value, label string) (uint64, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 || strconv.FormatUint(id, 10) != value {
		return 0, fmt.Errorf("%s必须为有效的正整数", label)
	}
	return id, nil
}

func validateTencentResource(kind domain.ResourceKind, action domain.ResourceAction, input domain.ResourceInput) error {
	if input.ID != "" {
		if _, err := tencentPositiveID(input.ID, "资源 ID"); err != nil {
			return err
		}
	}
	if action == domain.ResourceDelete || action == domain.ResourceQuery {
		return nil
	}
	if kind == domain.ResourceTemplates {
		if len(input.ProviderFields) != 0 {
			return fmt.Errorf("模板不支持签名申请字段")
		}
		_, err := (NativeTemplateCodec{ID: "tencent-native-v1", AllowNumeric: true}).Parse(input.Content, nil)
		return err
	}
	allowed := map[string]bool{"sign_type": true, "document_type": true, "sign_purpose": true, "qualification_id": true, "proof_image": true, "commission_image": true}
	for name := range input.ProviderFields {
		if !allowed[name] {
			return fmt.Errorf("不支持的签名申请字段：%s", name)
		}
	}
	f := input.ProviderFields
	valid := map[string]map[string]bool{"0": {"0": true, "1": true}, "4": {"7": true}, "5": {"2": true, "3": true}}
	if !valid[f["sign_type"]][f["document_type"]] {
		return fmt.Errorf("签名类型与证明类型不匹配")
	}
	if _, err := tencentPositiveID(f["qualification_id"], "资质 ID"); err != nil {
		return err
	}
	if f["sign_purpose"] != "0" && f["sign_purpose"] != "1" {
		return fmt.Errorf("请选择自用或他用签名")
	}
	if err := validateTencentProof(f["proof_image"], "资质证明图片"); err != nil {
		return err
	}
	if f["sign_purpose"] == "1" && f["commission_image"] == "" {
		return fmt.Errorf("他用签名必须提供委托授权证明")
	}
	if f["commission_image"] != "" {
		return validateTencentProof(f["commission_image"], "委托授权证明")
	}
	return nil
}

func validateTencentProof(value, label string) error {
	// Bound decoding locally; the cloud retains its own material requirements.
	if len(value) > 8<<20 || strings.HasPrefix(value, "data:") {
		return fmt.Errorf("%s需提供不含 data 前缀的 Base64 图片，编码后不超过 8 MiB", label)
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(data) == 0 {
		return fmt.Errorf("%s不是有效的 Base64 图片", label)
	}
	info, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || info.Width <= 0 || info.Height <= 0 {
		return fmt.Errorf("%s必须为 JPEG 或 PNG 图片", label)
	}
	return nil
}

func queryTencentResources(ctx context.Context, client *sms.Client, kind domain.ResourceKind, id string) ([]domain.RemoteResource, error) {
	var ids []*uint64
	if id != "" {
		value, err := tencentPositiveID(id, "资源 ID")
		if err != nil {
			return nil, err
		}
		ids = []*uint64{common.Uint64Ptr(value)}
	}
	items := []domain.RemoteResource{}
	seen := map[string]bool{}
	for offset := uint64(0); ; offset += 100 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page := []domain.RemoteResource{}
		if kind == domain.ResourceTemplates {
			req := sms.NewDescribeSmsTemplateListRequest()
			req.International, req.TemplateIdSet, req.Limit, req.Offset = common.Uint64Ptr(0), ids, common.Uint64Ptr(100), common.Uint64Ptr(offset)
			resp, err := client.DescribeSmsTemplateListWithContext(ctx, req)
			if err != nil {
				return nil, tencentFailure(err, false)
			}
			if resp == nil || resp.Response == nil {
				return nil, tencentInvalidResponse("", false)
			}
			requestID := tcValue(resp.Response.RequestId)
			for _, row := range resp.Response.DescribeTemplateStatusSet {
				if row == nil || tcValue(row.TemplateId) == 0 || row.TemplateContent == nil || row.International == nil {
					return nil, tencentInvalidResponse(requestID, false)
				}
				if *row.International != 0 && *row.International != 3 {
					continue
				}
				if id != "" && strconv.FormatUint(*row.TemplateId, 10) != id {
					return nil, tencentInvalidResponse(requestID, false)
				}
				page = append(page, domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: strconv.FormatUint(*row.TemplateId, 10), Name: tcValue(row.TemplateName), Content: *row.TemplateContent}, AuditStatus: tencentAudit(row.StatusCode), AuditReply: tcValue(row.ReviewReply), ProviderMetadata: tencentResourceMetadata(row.StatusCode), RequestID: requestID})
			}
			// Pagination follows the upstream page size, including out-of-scope rows.
			for _, item := range page {
				if seen[item.ID] {
					return nil, tencentInvalidResponse(requestID, false)
				}
				seen[item.ID] = true
				items = append(items, item)
			}
			if id != "" || len(resp.Response.DescribeTemplateStatusSet) < 100 {
				return items, nil
			}
		} else {
			req := sms.NewDescribeSmsSignListRequest()
			req.International, req.SignIdSet, req.Limit, req.Offset = common.Uint64Ptr(0), ids, common.Uint64Ptr(100), common.Uint64Ptr(offset)
			resp, err := client.DescribeSmsSignListWithContext(ctx, req)
			if err != nil {
				return nil, tencentFailure(err, false)
			}
			if resp == nil || resp.Response == nil {
				return nil, tencentInvalidResponse("", false)
			}
			requestID := tcValue(resp.Response.RequestId)
			for _, row := range resp.Response.DescribeSignListStatusSet {
				if row == nil || tcValue(row.SignId) == 0 || row.SignName == nil || row.International == nil {
					return nil, tencentInvalidResponse(requestID, false)
				}
				if *row.International != 0 {
					continue
				}
				if id != "" && strconv.FormatUint(*row.SignId, 10) != id {
					return nil, tencentInvalidResponse(requestID, false)
				}
				metadata := tencentResourceMetadata(row.StatusCode)
				if row.QualificationId != nil {
					metadata["qualification_id"] = strconv.FormatUint(*row.QualificationId, 10)
				}
				if row.QualificationName != nil {
					metadata["qualification_name"] = *row.QualificationName
				}
				if row.QualificationStatusCode != nil {
					metadata["qualification_status_code"] = *row.QualificationStatusCode
				}
				page = append(page, domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: strconv.FormatUint(*row.SignId, 10), Content: *row.SignName}, AuditStatus: tencentAudit(row.StatusCode), AuditReply: tcValue(row.ReviewReply), ProviderMetadata: metadata, RequestID: requestID})
			}
			for _, item := range page {
				if seen[item.ID] {
					return nil, tencentInvalidResponse(requestID, false)
				}
				seen[item.ID] = true
				items = append(items, item)
			}
			if id != "" || len(resp.Response.DescribeSignListStatusSet) < 100 {
				return items, nil
			}
		}
	}
}

func tencentResourceMetadata(status *int64) map[string]any {
	metadata := map[string]any{}
	if status != nil {
		metadata["status_code"] = *status
	}
	return metadata
}

func mutateTencentResource(ctx context.Context, c *sms.Client, kind domain.ResourceKind, action domain.ResourceAction, input domain.ResourceInput) ([]domain.RemoteResource, error) {
	id, _ := strconv.ParseUint(input.ID, 10, 64)
	var resultID uint64
	var requestID string
	if kind == domain.ResourceTemplates {
		category, _ := strconv.ParseUint(input.Category, 10, 64)
		switch action {
		case domain.ResourceCreate:
			req := sms.NewAddSmsTemplateRequest()
			req.TemplateName, req.TemplateContent, req.SmsType, req.International, req.Remark = common.StringPtr(input.Name), common.StringPtr(input.Content), &category, common.Uint64Ptr(0), common.StringPtr(input.Description)
			resp, err := c.AddSmsTemplateWithContext(ctx, req)
			if err != nil {
				return nil, tencentFailure(err, true)
			}
			if resp != nil && resp.Response != nil {
				requestID = tcValue(resp.Response.RequestId)
				if resp.Response.AddTemplateStatus != nil {
					resultID, _ = tencentPositiveID(tcValue(resp.Response.AddTemplateStatus.TemplateId), "模板 ID")
				}
			}
		case domain.ResourceUpdate:
			req := sms.NewModifySmsTemplateRequest()
			req.TemplateId, req.TemplateName, req.TemplateContent, req.SmsType, req.International, req.Remark = &id, common.StringPtr(input.Name), common.StringPtr(input.Content), &category, common.Uint64Ptr(0), common.StringPtr(input.Description)
			resp, err := c.ModifySmsTemplateWithContext(ctx, req)
			if err != nil {
				return nil, tencentFailure(err, true)
			}
			if resp != nil && resp.Response != nil {
				requestID = tcValue(resp.Response.RequestId)
				if resp.Response.ModifyTemplateStatus != nil {
					resultID = tcValue(resp.Response.ModifyTemplateStatus.TemplateId)
				}
			}
		case domain.ResourceDelete:
			req := sms.NewDeleteSmsTemplateRequest()
			req.TemplateId = &id
			resp, err := c.DeleteSmsTemplateWithContext(ctx, req)
			if err != nil {
				return nil, tencentFailure(err, true)
			}
			if resp != nil && resp.Response != nil {
				requestID = tcValue(resp.Response.RequestId)
				if resp.Response.DeleteTemplateStatus != nil && tcValue(resp.Response.DeleteTemplateStatus.DeleteStatus) == "return successfully!" {
					resultID = id
				}
			}
		}
	} else {
		f := input.ProviderFields
		signType, _ := strconv.ParseUint(f["sign_type"], 10, 64)
		documentType, _ := strconv.ParseUint(f["document_type"], 10, 64)
		purpose, _ := strconv.ParseUint(f["sign_purpose"], 10, 64)
		qualification, _ := strconv.ParseUint(f["qualification_id"], 10, 64)
		switch action {
		case domain.ResourceCreate:
			req := sms.NewAddSmsSignRequest()
			req.SignName, req.SignType, req.DocumentType, req.SignPurpose, req.International = common.StringPtr(input.Content), &signType, &documentType, &purpose, common.Uint64Ptr(0)
			req.QualificationId, req.ProofImage, req.Remark = &qualification, common.StringPtr(f["proof_image"]), common.StringPtr(input.Description)
			if purpose == 1 {
				req.CommissionImage = common.StringPtr(f["commission_image"])
			}
			resp, err := c.AddSmsSignWithContext(ctx, req)
			if err != nil {
				return nil, tencentFailure(err, true)
			}
			if resp != nil && resp.Response != nil {
				requestID = tcValue(resp.Response.RequestId)
				if resp.Response.AddSignStatus != nil {
					resultID = tcValue(resp.Response.AddSignStatus.SignId)
				}
			}
		case domain.ResourceUpdate:
			req := sms.NewModifySmsSignRequest()
			req.SignId, req.SignName, req.SignType, req.DocumentType, req.SignPurpose, req.International = &id, common.StringPtr(input.Content), &signType, &documentType, &purpose, common.Uint64Ptr(0)
			req.QualificationId, req.ProofImage, req.Remark = &qualification, common.StringPtr(f["proof_image"]), common.StringPtr(input.Description)
			if purpose == 1 {
				req.CommissionImage = common.StringPtr(f["commission_image"])
			}
			resp, err := c.ModifySmsSignWithContext(ctx, req)
			if err != nil {
				return nil, tencentFailure(err, true)
			}
			if resp != nil && resp.Response != nil {
				requestID = tcValue(resp.Response.RequestId)
				if resp.Response.ModifySignStatus != nil {
					resultID = tcValue(resp.Response.ModifySignStatus.SignId)
				}
			}
		case domain.ResourceDelete:
			req := sms.NewDeleteSmsSignRequest()
			req.SignId = &id
			resp, err := c.DeleteSmsSignWithContext(ctx, req)
			if err != nil {
				return nil, tencentFailure(err, true)
			}
			if resp != nil && resp.Response != nil {
				requestID = tcValue(resp.Response.RequestId)
				if resp.Response.DeleteSignStatus != nil && tcValue(resp.Response.DeleteSignStatus.DeleteStatus) == "return successfully!" {
					resultID = id
				}
			}
		}
	}
	if resultID == 0 || (action != domain.ResourceCreate && resultID != id) {
		return nil, tencentInvalidResponse(requestID, true)
	}
	return []domain.RemoteResource{{ResourceInput: domain.ResourceInput{ID: strconv.FormatUint(resultID, 10)}, RequestID: requestID}}, nil
}
