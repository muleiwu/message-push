package sender

import (
	"context"
	"encoding/json"
	"fmt"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/registry"
)

func init() {
	// 注册阿里云短信服务商
	registry.Register(&registry.ProviderMeta{
		Code:        constants.ProviderAliyunSMS,
		Name:        "阿里云短信",
		Type:        constants.MessageTypeSMS,
		Description: "阿里云短信服务，支持国内短信和国际短信发送。注意：短信签名需在「签名管理」中单独配置",
		ConfigFields: []registry.ConfigField{
			{
				Key:            "access_key_id",
				Label:          "AccessKeyID",
				Description:    "阿里云账号的AccessKeyID",
				Type:           registry.FieldTypeText,
				Required:       true,
				Example:        "LTAI5tXXXXXXXXXXXXXX",
				Placeholder:    "请输入AccessKeyID",
				ValidationRule: "min:16,max:64",
				HelpLink:       "https://help.aliyun.com/document_detail/53045.html",
			},
			{
				Key:            "access_key_secret",
				Label:          "AccessKeySecret",
				Description:    "阿里云账号的AccessKeySecret",
				Type:           registry.FieldTypePassword,
				Required:       true,
				Example:        "xxxxxxxxxxxxxxxxxxxxxx",
				Placeholder:    "请输入AccessKeySecret",
				ValidationRule: "min:16,max:64",
				HelpLink:       "https://help.aliyun.com/document_detail/53045.html",
			},
		},
	})
}

// AliyunSMSSender 阿里云短信发送器
type AliyunSMSSender struct {
}

// NewAliyunSMSSender 创建阿里云短信发送器
func NewAliyunSMSSender() *AliyunSMSSender {
	return &AliyunSMSSender{}
}

// GetType 获取发送器类型
func (s *AliyunSMSSender) GetType() string {
	return constants.MessageTypeSMS
}

// Send 发送短信
func (s *AliyunSMSSender) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	// 解析服务商配置
	var config struct {
		AccessKeyID     string `json:"access_key_id"`
		AccessKeySecret string `json:"access_key_secret"`
	}

	if err := json.Unmarshal([]byte(req.Provider.Config), &config); err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	// 从签名表获取默认签名
	signName := ""
	if req.Signature != nil {
		signName = req.Signature.SignatureCode
	}

	if signName == "" {
		return nil, fmt.Errorf("no signature configured for this provider account")
	}

	// 解析通道配置（模板ID等）
	var channelConfig struct {
		TemplateCode string `json:"template_code"`
	}
	if req.ProviderChannel.Config != "" {
		json.Unmarshal([]byte(req.ProviderChannel.Config), &channelConfig)
	}

	// TODO: 实际调用阿里云SDK
	// import "github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
	//
	// client, err := dysmsapi.NewClientWithAccessKey("cn-hangzhou", config.AccessKeyID, config.AccessKeySecret)
	// if err != nil {
	//     return nil, err
	// }
	//
	// request := dysmsapi.CreateSendSmsRequest()
	// request.PhoneNumbers = req.Task.Receiver
	// request.SignName = signName
	// request.TemplateCode = channelConfig.TemplateCode
	// request.TemplateParam = req.Task.TemplateParams
	//
	// response, err := client.SendSms(request)
	// if err != nil {
	//     return &SendResponse{
	//         Success:      false,
	//         ErrorMessage: err.Error(),
	//     }, nil
	// }
	//
	// if response.Code != "OK" {
	//     return &SendResponse{
	//         Success:      false,
	//         ErrorCode:    response.Code,
	//         ErrorMessage: response.Message,
	//     }, nil
	// }
	//
	// return &SendResponse{
	//     Success:    true,
	//     ProviderID: response.BizId,
	// }, nil

	// 模拟发送（开发阶段）
	return &SendResponse{
		Success:    true,
		ProviderID: fmt.Sprintf("mock_aliyun_%s_%s", signName, req.Task.TaskID),
	}, nil
}
