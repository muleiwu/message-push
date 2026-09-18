package infrastructure

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/alibabacloud-go/tea/tea"
)

const aliyunSMSEndpoint = "dysmsapi.aliyuncs.com"

type aliyunClientFactory func(*model.ProviderAccount) (*dysmsapi.Client, error)

func newAliyunClient(account *model.ProviderAccount) (*dysmsapi.Client, error) {
	if account == nil {
		return nil, fmt.Errorf("供应商账号不存在")
	}
	config, err := account.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("阿里云账号配置无效")
	}
	id, _ := config["access_key_id"].(string)
	secret, _ := config["access_key_secret"].(string)
	if id == "" || secret == "" {
		return nil, fmt.Errorf("请先配置阿里云 AccessKeyID 和 AccessKeySecret")
	}
	return dysmsapi.NewClient(&openapi.Config{
		AccessKeyId: tea.String(id), AccessKeySecret: tea.String(secret),
		Endpoint: tea.String(aliyunSMSEndpoint), Protocol: tea.String("https"),
		RegionId: tea.String("cn-hangzhou"), ConnectTimeout: tea.Int(5000), ReadTimeout: tea.Int(10000),
		RetryOptions: &dara.RetryOptions{Retryable: false},
	})
}

func (s *AliyunSMSSender) accountClient(account *model.ProviderAccount) (*dysmsapi.Client, error) {
	if s.clientFactory != nil {
		return s.clientFactory(account)
	}
	return newAliyunClient(account)
}

func aliyunRuntime() *dara.RuntimeOptions {
	return &dara.RuntimeOptions{Autoretry: tea.Bool(false), MaxAttempts: tea.Int(1), ConnectTimeout: tea.Int(5000), ReadTimeout: tea.Int(10000)}
}

func aliyunFailure(err error, writing bool) error {
	if err == nil {
		return nil
	}
	result := &domain.RemoteResourceError{Code: "TRANSPORT_ERROR", Message: "阿里云请求未完成", Uncertain: writing,
		Diagnostic: domain.ResourceDiagnostic{URL: "https://" + aliyunSMSEndpoint + "/", Method: "POST", Cause: err.Error()}}
	var sdk *tea.SDKError
	if errors.As(err, &sdk) {
		result.Code, result.Message = tea.StringValue(sdk.Code), "阿里云："+tea.StringValue(sdk.Message)
		result.Diagnostic.HTTPStatus = tea.IntValue(sdk.StatusCode)
		result.Diagnostic.ResponseBody = tea.StringValue(sdk.Data)
		var data map[string]any
		if json.Unmarshal([]byte(tea.StringValue(sdk.Data)), &data) == nil {
			result.RequestID, _ = data["RequestId"].(string)
		}
		result.Uncertain = writing && (result.Diagnostic.HTTPStatus >= 500 || aliyunUncertainCode(result.Code))
	}
	return result
}

func aliyunUncertainCode(code string) bool {
	code = strings.NewReplacer("_", "", ".", "", "-", "").Replace(strings.ToLower(code))
	return strings.Contains(code, "internal") || strings.Contains(code, "systemerror") || strings.Contains(code, "timeout") || strings.Contains(code, "serviceunavailable")
}

func aliyunResponseError(code, message, requestID *string, writing bool) error {
	if code == nil || *code == "" {
		return aliyunInvalidResponse(tea.StringValue(requestID), writing)
	}
	if *code == "OK" {
		return nil
	}
	return &domain.RemoteResourceError{Code: *code, Message: "阿里云：" + tea.StringValue(message), RequestID: tea.StringValue(requestID), Uncertain: writing && aliyunUncertainCode(*code)}
}

func aliyunInvalidResponse(requestID string, writing bool) error {
	return &domain.RemoteResourceError{Code: "INVALID_RESPONSE", Message: "阿里云响应缺少有效资源数据，请查询确认结果", RequestID: requestID, Uncertain: writing}
}

func aliyunAudit(raw string) int8 {
	switch raw {
	case "0", "AUDIT_STATE_INIT":
		return 1
	case "1", "AUDIT_STATE_PASS":
		return 2
	case "2", "AUDIT_STATE_NOT_PASS":
		return 3
	default:
		return 0
	}
}

func aliyunMetadata(raw, orderID string) map[string]any {
	return map[string]any{"aliyun": map[string]any{"audit_status": raw, "order_id": orderID}}
}

func aliyunOrderID(resource domain.RemoteResource) string {
	metadata, _ := resource.ProviderMetadata["aliyun"].(map[string]any)
	id, _ := metadata["order_id"].(string)
	return id
}
