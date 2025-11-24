package sender

import (
	"context"
	"encoding/json"
	"fmt"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/registry"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sms "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sms/v20210111"
)

func init() {
	// 注册腾讯云短信服务商
	registry.Register(&registry.ProviderMeta{
		Code:        constants.ProviderTencentSMS,
		Name:        "腾讯云短信",
		Type:        constants.MessageTypeSMS,
		Description: "腾讯云短信服务，支持国内短信和国际短信发送。注意：短信签名需在「签名管理」中单独配置",
		ConfigFields: []registry.ConfigField{
			{
				Key:            "secret_id",
				Label:          "SecretId",
				Description:    "腾讯云账号的SecretId",
				Type:           registry.FieldTypeText,
				Required:       true,
				Example:        "AKIDxxxxxxxxxxxxxxxx",
				Placeholder:    "请输入SecretId",
				ValidationRule: "min:16,max:64",
				HelpLink:       "https://cloud.tencent.com/document/product/382/37794",
			},
			{
				Key:            "secret_key",
				Label:          "SecretKey",
				Description:    "腾讯云账号的SecretKey",
				Type:           registry.FieldTypePassword,
				Required:       true,
				Example:        "xxxxxxxxxxxxxxxxxxxxxx",
				Placeholder:    "请输入SecretKey",
				ValidationRule: "min:16,max:64",
				HelpLink:       "https://cloud.tencent.com/document/product/382/37794",
			},
			{
				Key:         "sdk_app_id",
				Label:       "应用ID",
				Description: "短信应用的SdkAppId",
				Type:        registry.FieldTypeText,
				Required:    true,
				Example:     "1400000000",
				Placeholder: "请输入SdkAppId",
			},
			{
				Key:          "region",
				Label:        "地域",
				Description:  "腾讯云地域，默认为ap-guangzhou",
				Type:         registry.FieldTypeText,
				Required:     false,
				Example:      "ap-guangzhou",
				Placeholder:  "请输入地域",
				DefaultValue: "ap-guangzhou",
			},
		},
	})
}

type TencentSMSSender struct {
}

func NewTencentSMSSender() *TencentSMSSender {
	return &TencentSMSSender{}
}

func (s *TencentSMSSender) GetType() string {
	return constants.MessageTypeSMS
}

func (s *TencentSMSSender) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	// 1. 获取配置
	config, err := req.Provider.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	secretId, _ := config["secret_id"].(string)
	secretKey, _ := config["secret_key"].(string)
	region, _ := config["region"].(string)
	sdkAppId, _ := config["sdk_app_id"].(string)

	if secretId == "" || secretKey == "" || sdkAppId == "" {
		return nil, fmt.Errorf("missing tencent sms config: secret_id, secret_key or sdk_app_id")
	}
	if region == "" {
		region = "ap-guangzhou"
	}

	// 2. 初始化客户端
	credential := common.NewCredential(secretId, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "sms.tencentcloudapi.com"
	client, _ := sms.NewClient(credential, region, cpf)

	// 3. 构造请求
	request := sms.NewSendSmsRequest()
	request.SmsSdkAppId = common.StringPtr(sdkAppId)

	// 签名和模板
	// 优先使用通道关联配置，其次使用请求中的签名
	signName := ""
	templateID := ""

	if req.Relation != nil {
		signName = req.Relation.SignatureCode
		templateID = req.Relation.TemplateID
	}

	if signName == "" {
		// 如果通道未配置，从请求中获取签名（可能为空）
		if req.Signature != nil {
			signName = req.Signature.SignatureCode
		}
	}

	if templateID == "" {
		// 如果通道未配置，尝试使用Task中的TemplateCode（假设Task的TemplateCode即为服务商TemplateID）
		templateID = req.Task.TemplateCode
	}

	if templateID == "" {
		return nil, fmt.Errorf("missing template_id")
	}

	request.SignName = common.StringPtr(signName)
	request.TemplateId = common.StringPtr(templateID)

	// 接收者
	request.PhoneNumberSet = common.StringPtrs([]string{req.Task.Receiver})

	// 模板参数
	var params []string
	if req.Task.TemplateParams != "" {
		// 尝试解析为 []string
		if err := json.Unmarshal([]byte(req.Task.TemplateParams), &params); err != nil {
			return nil, fmt.Errorf("invalid template_params for tencent sms, expected json array: %v", err)
		}
	}
	request.TemplateParamSet = common.StringPtrs(params)

	// 4. 发送
	response, err := client.SendSms(request)
	if err != nil {
		return nil, err
	}

	// 5. 解析响应
	// 腾讯云支持批量发送，这里我们只发了一条
	if len(response.Response.SendStatusSet) > 0 {
		status := response.Response.SendStatusSet[0]
		if *status.Code == "Ok" {
			return &SendResponse{
				Success:    true,
				ProviderID: *status.SerialNo,
			}, nil
		}
		return &SendResponse{
			Success:      false,
			ErrorCode:    *status.Code,
			ErrorMessage: *status.Message,
		}, nil
	}

	return &SendResponse{
		Success:      false,
		ErrorMessage: "empty response from tencent cloud",
	}, nil
}
