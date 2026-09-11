package infrastructure

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/tea"
)

func queryAliyunResources(ctx context.Context, client *dysmsapi.Client, kind domain.ResourceKind, id string) ([]domain.RemoteResource, error) {
	if id != "" {
		row, err := getAliyunResource(ctx, client, kind, id)
		if err != nil {
			return nil, err
		}
		return []domain.RemoteResource{row}, nil
	}
	items := []domain.RemoteResource{}
	seen := map[string]bool{}
	var received, expected int64
	for page := int32(1); ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, count, total, requestID, err := listAliyunResourcePage(ctx, client, kind, page)
		if err != nil {
			return nil, err
		}
		if page == 1 {
			expected = total
		}
		if total != expected || total < 0 || count > 50 {
			return nil, aliyunInvalidResponse(requestID, false)
		}
		for _, row := range rows {
			if seen[row.ID] {
				return nil, aliyunInvalidResponse(requestID, false)
			}
			seen[row.ID] = true
			if kind == domain.ResourceTemplates && row.Category == "3" {
				continue
			}
			items = append(items, row)
		}
		received += int64(count)
		if received == total {
			return items, nil
		}
		if received > total || count < 50 {
			return nil, aliyunInvalidResponse(requestID, false)
		}
	}
}

func listAliyunResourcePage(ctx context.Context, client *dysmsapi.Client, kind domain.ResourceKind, page int32) ([]domain.RemoteResource, int, int64, string, error) {
	rows := []domain.RemoteResource{}
	if kind == domain.ResourceTemplates {
		resp, err := client.QuerySmsTemplateListWithContext(ctx, &dysmsapi.QuerySmsTemplateListRequest{PageIndex: &page, PageSize: tea.Int32(50)}, aliyunRuntime())
		if err != nil {
			return nil, 0, 0, "", aliyunFailure(err, false)
		}
		if resp == nil || resp.Body == nil {
			return nil, 0, 0, "", aliyunInvalidResponse("", false)
		}
		b := resp.Body
		requestID := tea.StringValue(b.RequestId)
		if err := aliyunResponseError(b.Code, b.Message, b.RequestId, false); err != nil {
			return nil, 0, 0, requestID, err
		}
		if b.TotalCount == nil || b.CurrentPage != nil && *b.CurrentPage != page {
			return nil, 0, 0, requestID, aliyunInvalidResponse(requestID, false)
		}
		for _, row := range b.SmsTemplateList {
			if row == nil || tea.StringValue(row.TemplateCode) == "" || strings.TrimSpace(tea.StringValue(row.TemplateContent)) == "" {
				return nil, 0, 0, requestID, aliyunInvalidResponse(requestID, false)
			}
			category := ""
			if row.OuterTemplateType != nil {
				category = strconv.FormatInt(int64(*row.OuterTemplateType), 10)
			} else if row.TemplateType != nil {
				category = map[int32]string{2: "0", 0: "1", 1: "2", 6: "3"}[*row.TemplateType]
			}
			if category != "0" && category != "1" && category != "2" && category != "3" {
				return nil, 0, 0, requestID, aliyunInvalidResponse(requestID, false)
			}
			raw := tea.StringValue(row.AuditStatus)
			item := domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: *row.TemplateCode, Name: tea.StringValue(row.TemplateName), Content: *row.TemplateContent, Category: category, ProviderFields: map[string]string{}}, AuditStatus: aliyunAudit(raw), ProviderMetadata: aliyunMetadata(raw, tea.StringValue(row.OrderId)), RequestID: requestID}
			if row.Reason != nil {
				item.AuditReply = aliyunRejectInfo(row.Reason.RejectInfo, row.Reason.RejectSubInfo)
			}
			if row.SignatureName != nil {
				item.ProviderFields["related_sign_name"] = *row.SignatureName
			}
			rows = append(rows, item)
		}
		return rows, len(b.SmsTemplateList), *b.TotalCount, requestID, nil
	}
	resp, err := client.QuerySmsSignListWithContext(ctx, &dysmsapi.QuerySmsSignListRequest{PageIndex: &page, PageSize: tea.Int32(50)}, aliyunRuntime())
	if err != nil {
		return nil, 0, 0, "", aliyunFailure(err, false)
	}
	if resp == nil || resp.Body == nil {
		return nil, 0, 0, "", aliyunInvalidResponse("", false)
	}
	b := resp.Body
	requestID := tea.StringValue(b.RequestId)
	if err := aliyunResponseError(b.Code, b.Message, b.RequestId, false); err != nil {
		return nil, 0, 0, requestID, err
	}
	if b.TotalCount == nil || b.CurrentPage != nil && *b.CurrentPage != page {
		return nil, 0, 0, requestID, aliyunInvalidResponse(requestID, false)
	}
	for _, row := range b.SmsSignList {
		if row == nil || strings.TrimSpace(tea.StringValue(row.SignName)) == "" {
			return nil, 0, 0, requestID, aliyunInvalidResponse(requestID, false)
		}
		raw := tea.StringValue(row.AuditStatus)
		item := domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: *row.SignName, Content: *row.SignName, ProviderFields: map[string]string{}}, AuditStatus: aliyunAudit(raw), ProviderMetadata: aliyunMetadata(raw, tea.StringValue(row.OrderId)), RequestID: requestID}
		meta := item.ProviderMetadata["aliyun"].(map[string]any)
		if row.BusinessType != nil {
			meta["business_type"] = *row.BusinessType
		}
		if row.Reason != nil {
			item.AuditReply = aliyunRejectInfo(row.Reason.RejectInfo, row.Reason.RejectSubInfo)
		}
		aliyunResourceIDField(&item, "authorization_letter_id", row.AuthorizationLetterId)
		aliyunResourceIDField(&item, "trademark_id", row.TrademarkId)
		if row.AuthorizationLetterAuditPass != nil {
			meta["authorization_letter_audit_pass"] = *row.AuthorizationLetterAuditPass
		}
		rows = append(rows, item)
	}
	return rows, len(b.SmsSignList), *b.TotalCount, requestID, nil
}

func getAliyunResource(ctx context.Context, client *dysmsapi.Client, kind domain.ResourceKind, id string) (domain.RemoteResource, error) {
	empty := domain.RemoteResource{}
	if kind == domain.ResourceTemplates {
		resp, err := client.GetSmsTemplateWithContext(ctx, &dysmsapi.GetSmsTemplateRequest{TemplateCode: &id}, aliyunRuntime())
		if err != nil {
			return empty, aliyunFailure(err, false)
		}
		if resp == nil || resp.Body == nil {
			return empty, aliyunInvalidResponse("", false)
		}
		b := resp.Body
		requestID := tea.StringValue(b.RequestId)
		if err := aliyunResponseError(b.Code, b.Message, b.RequestId, false); err != nil {
			return empty, err
		}
		if tea.StringValue(b.TemplateCode) != id || strings.TrimSpace(tea.StringValue(b.TemplateContent)) == "" {
			return empty, aliyunInvalidResponse(requestID, false)
		}
		category := tea.StringValue(b.TemplateType)
		if category != "0" && category != "1" && category != "2" {
			return empty, &domain.RemoteResourceError{Code: "UNSUPPORTED_RESOURCE", Message: "仅支持管理国内短信模板", RequestID: requestID}
		}
		raw := tea.StringValue(b.TemplateStatus)
		item := domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: id, Name: tea.StringValue(b.TemplateName), Content: *b.TemplateContent, Category: category, Description: tea.StringValue(b.Remark), ProviderFields: map[string]string{}}, AuditStatus: aliyunAudit(raw), ProviderMetadata: aliyunMetadata(raw, tea.StringValue(b.OrderId)), RequestID: requestID}
		if b.AuditInfo != nil {
			item.AuditReply = tea.StringValue(b.AuditInfo.RejectInfo)
		}
		for name, value := range map[string]*string{"related_sign_name": b.RelatedSignName, "template_rule": b.VariableAttribute, "apply_scene_content": b.ApplyScene} {
			if value != nil {
				item.ProviderFields[name] = *value
				item.ProviderMetadata["aliyun"].(map[string]any)[name] = *value
			}
		}
		return item, nil
	}
	resp, err := client.GetSmsSignWithContext(ctx, &dysmsapi.GetSmsSignRequest{SignName: &id}, aliyunRuntime())
	if err != nil {
		return empty, aliyunFailure(err, false)
	}
	if resp == nil || resp.Body == nil {
		return empty, aliyunInvalidResponse("", false)
	}
	b := resp.Body
	requestID := tea.StringValue(b.RequestId)
	if err := aliyunResponseError(b.Code, b.Message, b.RequestId, false); err != nil {
		return empty, err
	}
	if tea.StringValue(b.SignName) != id {
		return empty, aliyunInvalidResponse(requestID, false)
	}
	raw := ""
	if b.SignStatus != nil {
		raw = strconv.FormatInt(*b.SignStatus, 10)
	}
	item := domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: id, Content: id, Description: tea.StringValue(b.Remark), ProviderFields: map[string]string{}}, AuditStatus: aliyunAudit(raw), ProviderMetadata: aliyunMetadata(raw, tea.StringValue(b.OrderId)), RequestID: requestID}
	meta := item.ProviderMetadata["aliyun"].(map[string]any)
	if b.AuditInfo != nil {
		item.AuditReply = tea.StringValue(b.AuditInfo.RejectInfo)
	}
	aliyunResourceIDField(&item, "qualification_id", b.QualificationId)
	aliyunResourceIDField(&item, "authorization_letter_id", b.AuthorizationLetterId)
	aliyunResourceIDField(&item, "trademark_id", b.TrademarkId)
	if b.ThirdParty != nil {
		item.ProviderFields["third_party"] = strconv.FormatBool(*b.ThirdParty)
		meta["third_party"] = *b.ThirdParty
	}
	if b.AuthorizationLetterAuditPass != nil {
		meta["authorization_letter_audit_pass"] = *b.AuthorizationLetterAuditPass
	}
	if b.SignCode != nil {
		meta["sign_code"] = *b.SignCode
	}
	if b.SignUsage != nil {
		meta["sign_usage"] = *b.SignUsage
	}
	// Material URLs can contain expiring credentials. Only retain public review facts.
	if b.SignIspRegisterDetailList != nil {
		encoded, _ := json.Marshal(b.SignIspRegisterDetailList)
		var details []map[string]any
		if json.Unmarshal(encoded, &details) == nil {
			meta["isp_registration"] = details
		}
	}
	return item, nil
}

func aliyunResourceIDField(item *domain.RemoteResource, key string, value *int64) {
	if value != nil && *value > 0 {
		text := strconv.FormatInt(*value, 10)
		item.ProviderFields[key] = text
		item.ProviderMetadata["aliyun"].(map[string]any)[key] = text
	}
}

func aliyunRejectInfo(info, detail *string) string {
	result := tea.StringValue(info)
	if tea.StringValue(detail) != "" && tea.StringValue(detail) != result {
		if result != "" {
			result += "；"
		}
		result += *detail
	}
	return result
}
