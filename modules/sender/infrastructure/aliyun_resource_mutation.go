package infrastructure

import (
	"context"
	"strconv"

	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/tea"
)

func mutateAliyunResource(ctx context.Context, client *dysmsapi.Client, kind domain.ResourceKind, action domain.ResourceAction, input domain.ResourceInput) ([]domain.RemoteResource, error) {
	var code, message, requestID, id, orderID *string
	materials, err := aliyunMaterialReferences(input.ProviderFields["more_data"])
	if err != nil {
		return nil, err
	}
	if kind == domain.ResourceTemplates {
		typeID, _ := strconv.ParseInt(input.Category, 10, 32)
		req := &dysmsapi.CreateSmsTemplateRequest{
			TemplateName: &input.Name, TemplateContent: &input.Content, TemplateType: tea.Int32(int32(typeID)), Remark: &input.Description,
			RelatedSignName: aliyunOptional(input.ProviderFields["related_sign_name"]), TemplateRule: aliyunOptional(input.ProviderFields["template_rule"]),
			ApplySceneContent: aliyunOptional(input.ProviderFields["apply_scene_content"]), MoreData: materials, TrafficDriving: aliyunOptional(input.ProviderFields["traffic_driving"]),
		}
		switch action {
		case domain.ResourceCreate:
			resp, err := client.CreateSmsTemplateWithContext(ctx, req, aliyunRuntime())
			if err != nil {
				return nil, aliyunFailure(err, true)
			}
			if resp == nil || resp.Body == nil {
				return nil, aliyunInvalidResponse("", true)
			}
			b := resp.Body
			code, message, requestID, id, orderID = b.Code, b.Message, b.RequestId, b.TemplateCode, b.OrderId
		case domain.ResourceUpdate:
			resp, err := client.UpdateSmsTemplateWithContext(ctx, &dysmsapi.UpdateSmsTemplateRequest{
				TemplateCode: &input.ID, TemplateName: req.TemplateName, TemplateContent: req.TemplateContent, TemplateType: req.TemplateType,
				Remark: req.Remark, RelatedSignName: req.RelatedSignName, TemplateRule: req.TemplateRule, ApplySceneContent: req.ApplySceneContent, MoreData: req.MoreData, TrafficDriving: req.TrafficDriving,
			}, aliyunRuntime())
			if err != nil {
				return nil, aliyunFailure(err, true)
			}
			if resp == nil || resp.Body == nil {
				return nil, aliyunInvalidResponse("", true)
			}
			b := resp.Body
			code, message, requestID, id, orderID = b.Code, b.Message, b.RequestId, b.TemplateCode, b.OrderId
		case domain.ResourceDelete:
			resp, err := client.DeleteSmsTemplateWithContext(ctx, &dysmsapi.DeleteSmsTemplateRequest{TemplateCode: &input.ID}, aliyunRuntime())
			if err != nil {
				return nil, aliyunFailure(err, true)
			}
			if resp == nil || resp.Body == nil {
				return nil, aliyunInvalidResponse("", true)
			}
			code, message, requestID, id = resp.Body.Code, resp.Body.Message, resp.Body.RequestId, &input.ID
		}
	} else {
		signType := int32(1)
		if input.ProviderFields["sign_type"] == "0" {
			signType = 0
		}
		signSource, _ := strconv.ParseInt(input.ProviderFields["sign_source"], 10, 32)
		req := &dysmsapi.CreateSmsSignRequest{
			SignName: &input.Content, Remark: &input.Description, SignSource: tea.Int32(int32(signSource)), SignType: &signType,
			ThirdParty: tea.Bool(input.ProviderFields["third_party"] == "true"), QualificationId: aliyunOptionalID(input.ProviderFields["qualification_id"]),
			AuthorizationLetterId: aliyunOptionalID(input.ProviderFields["authorization_letter_id"]), TrademarkId: aliyunOptionalID(input.ProviderFields["trademark_id"]), MoreData: materials,
		}
		switch action {
		case domain.ResourceCreate:
			resp, err := client.CreateSmsSignWithContext(ctx, req, aliyunRuntime())
			if err != nil {
				return nil, aliyunFailure(err, true)
			}
			if resp == nil || resp.Body == nil {
				return nil, aliyunInvalidResponse("", true)
			}
			b := resp.Body
			code, message, requestID, id, orderID = b.Code, b.Message, b.RequestId, b.SignName, b.OrderId
		case domain.ResourceUpdate:
			resp, err := client.UpdateSmsSignWithContext(ctx, &dysmsapi.UpdateSmsSignRequest{
				SignName: &input.ID, Remark: req.Remark, SignSource: req.SignSource, SignType: req.SignType, ThirdParty: req.ThirdParty,
				QualificationId: req.QualificationId, AuthorizationLetterId: req.AuthorizationLetterId, TrademarkId: req.TrademarkId, MoreData: req.MoreData,
			}, aliyunRuntime())
			if err != nil {
				return nil, aliyunFailure(err, true)
			}
			if resp == nil || resp.Body == nil {
				return nil, aliyunInvalidResponse("", true)
			}
			b := resp.Body
			code, message, requestID, id, orderID = b.Code, b.Message, b.RequestId, b.SignName, b.OrderId
		case domain.ResourceDelete:
			resp, err := client.DeleteSmsSignWithContext(ctx, &dysmsapi.DeleteSmsSignRequest{SignName: &input.ID}, aliyunRuntime())
			if err != nil {
				return nil, aliyunFailure(err, true)
			}
			if resp == nil || resp.Body == nil {
				return nil, aliyunInvalidResponse("", true)
			}
			b := resp.Body
			code, message, requestID, id = b.Code, b.Message, b.RequestId, b.SignName
			if id == nil {
				id = &input.ID
			}
		}
	}
	if err := aliyunResponseError(code, message, requestID, true); err != nil {
		return nil, err
	}
	if tea.StringValue(id) == "" || action != domain.ResourceCreate && *id != input.ID || kind == domain.ResourceSignatures && action == domain.ResourceCreate && *id != input.Content {
		return nil, aliyunInvalidResponse(tea.StringValue(requestID), true)
	}
	if action != domain.ResourceDelete && tea.StringValue(orderID) == "" {
		return nil, aliyunInvalidResponse(tea.StringValue(requestID), true)
	}
	return []domain.RemoteResource{{ResourceInput: domain.ResourceInput{ID: *id}, RequestID: tea.StringValue(requestID), ProviderMetadata: aliyunMetadata("", tea.StringValue(orderID))}}, nil
}

func aliyunOptional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func aliyunOptionalID(value string) *int64 {
	if value == "" {
		return nil
	}
	id, _ := strconv.ParseInt(value, 10, 64)
	return &id
}
