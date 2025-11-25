package sender

import (
	"context"
	"encoding/json"
	"fmt"

	"cnb.cool/mliev/push/message-push/app/constants"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
)

// AliyunSMSSenderReal 阿里云SMS真实发送器
type AliyunSMSSenderReal struct {
	client     *dysmsapi.Client
	signName   string
	templateID string
}

// AliyunSMSConfig 阿里云SMS配置
type AliyunSMSConfig struct {
	AccessKeyID     string `json:"access_key_id"`
	AccessKeySecret string `json:"access_key_secret"`
	SignName        string `json:"sign_name"`
	TemplateCode    string `json:"template_code"`
	Region          string `json:"region"`
}

// NewAliyunSMSSenderReal 创建阿里云SMS真实发送器
func NewAliyunSMSSenderReal(config map[string]interface{}) (Sender, error) {
	// 解析配置
	configJSON, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("invalid config format: %w", err)
	}

	var smsConfig AliyunSMSConfig
	if err := json.Unmarshal(configJSON, &smsConfig); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// 验证必需配置
	if smsConfig.AccessKeyID == "" || smsConfig.AccessKeySecret == "" {
		return nil, fmt.Errorf("access_key_id and access_key_secret are required")
	}

	// 默认区域
	if smsConfig.Region == "" {
		smsConfig.Region = "cn-hangzhou"
	}

	// 创建阿里云客户端
	openAPIConfig := &openapi.Config{
		AccessKeyId:     tea.String(smsConfig.AccessKeyID),
		AccessKeySecret: tea.String(smsConfig.AccessKeySecret),
		Endpoint:        tea.String("dysmsapi.aliyuncs.com"),
	}

	client, err := dysmsapi.NewClient(openAPIConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create aliyun sms client: %w", err)
	}

	return &AliyunSMSSenderReal{
		client:     client,
		signName:   smsConfig.SignName,
		templateID: smsConfig.TemplateCode,
	}, nil
}

// Send 发送短信
func (s *AliyunSMSSenderReal) Send(ctx context.Context, request *SendRequest) (*SendResponse, error) {
	// 从 Task 中获取接收者和模板参数
	task := request.Task
	if task == nil {
		return nil, fmt.Errorf("task is required")
	}

	// 准备模板参数
	templateParam := ""
	if task.TemplateParams != "" {
		// 模板参数已经是 JSON 字符串格式
		templateParam = task.TemplateParams
	}

	// 获取模板和签名
	templateCode := s.templateID
	if task.TemplateCode != "" {
		templateCode = task.TemplateCode
	}

	signName := s.signName
	// 从 ProviderAccount config 中获取签名（如果有）
	if request.ProviderAccount != nil && request.ProviderAccount.Config != "" {
		var config AliyunSMSConfig
		if err := json.Unmarshal([]byte(request.ProviderAccount.Config), &config); err == nil {
			if config.SignName != "" {
				signName = config.SignName
			}
		}
	}

	// 构建发送请求
	sendRequest := &dysmsapi.SendSmsRequest{
		PhoneNumbers:  tea.String(task.Receiver),
		SignName:      tea.String(signName),
		TemplateCode:  tea.String(templateCode),
		TemplateParam: tea.String(templateParam),
	}

	// 发送短信
	runtime := &util.RuntimeOptions{}
	response, err := s.client.SendSmsWithOptions(sendRequest, runtime)
	if err != nil {
		return &SendResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to send sms: %v", err),
		}, nil
	}

	// 检查响应
	if response.Body.Code == nil || *response.Body.Code != "OK" {
		errCode := "UNKNOWN"
		errMsg := "unknown error"
		if response.Body.Code != nil {
			errCode = *response.Body.Code
		}
		if response.Body.Message != nil {
			errMsg = *response.Body.Message
		}
		return &SendResponse{
			Success:      false,
			ErrorCode:    errCode,
			ErrorMessage: errMsg,
		}, nil
	}

	// 发送成功
	return &SendResponse{
		Success:    true,
		ProviderID: tea.StringValue(response.Body.BizId),
	}, nil
}

// GetProviderCode 获取服务商代码
func (s *AliyunSMSSenderReal) GetProviderCode() string {
	return constants.ProviderAliyunSMS
}

// Validate 验证配置
func (s *AliyunSMSSenderReal) Validate() error {
	if s.client == nil {
		return fmt.Errorf("aliyun sms client not initialized")
	}
	return nil
}
