package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
		// 能力声明
		SupportsSend:      true,
		SupportsBatchSend: true,
		SupportsCallback:  true,
		// 扩展信息
		Website:    "https://www.aliyun.com/product/sms",
		Icon:       "/image/logo/alibabacloud-color.png",
		DocsUrl:    "https://help.aliyun.com/document_detail/419273.html",
		ConsoleUrl: "https://dysms.console.aliyun.com/",
		PricingUrl: "https://www.aliyun.com/price/product#/sms/detail",
		SortOrder:  10,
		Tags:       []string{"国内", "国际", "推荐"},
		Regions:    []string{"中国大陆", "国际"},
		Deprecated: false,
	})
}

// AliyunSMSSender 阿里云短信发送器
type AliyunSMSSender struct {
}

// NewAliyunSMSSender 创建阿里云短信发送器
func NewAliyunSMSSender() *AliyunSMSSender {
	return &AliyunSMSSender{}
}

// GetProviderCode 获取服务商代码
func (s *AliyunSMSSender) GetProviderCode() string {
	return constants.ProviderAliyunSMS
}

// Send 发送短信
func (s *AliyunSMSSender) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	// 解析服务商配置
	var config struct {
		AccessKeyID     string `json:"access_key_id"`
		AccessKeySecret string `json:"access_key_secret"`
	}

	if err := json.Unmarshal([]byte(req.ProviderAccount.Config), &config); err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	// 从请求中获取签名（可能为空）
	signName := ""
	if req.Signature != nil {
		signName = req.Signature.SignatureCode
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
		TaskID:     req.Task.TaskID,
	}, nil
}

// ==================== BatchSender 接口实现 ====================

// SupportsBatchSend 是否支持批量发送
func (s *AliyunSMSSender) SupportsBatchSend() bool {
	return true
}

// BatchSend 批量发送短信（阿里云支持最多1000个号码）
func (s *AliyunSMSSender) BatchSend(ctx context.Context, req *BatchSendRequest) (*BatchSendResponse, error) {
	if len(req.Tasks) == 0 {
		return &BatchSendResponse{Results: []*SendResponse{}}, nil
	}

	// 解析服务商配置
	var config struct {
		AccessKeyID     string `json:"access_key_id"`
		AccessKeySecret string `json:"access_key_secret"`
	}

	if err := json.Unmarshal([]byte(req.ProviderAccount.Config), &config); err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	// 从请求中获取签名
	signName := ""
	if req.Signature != nil {
		signName = req.Signature.SignatureCode
	}

	// 收集所有手机号
	var phoneNumbers []string
	for _, task := range req.Tasks {
		phoneNumbers = append(phoneNumbers, task.Receiver)
	}

	// TODO: 实际调用阿里云批量发送SDK
	// import "github.com/aliyun/alibaba-cloud-sdk-go/services/dysmsapi"
	//
	// client, err := dysmsapi.NewClientWithAccessKey("cn-hangzhou", config.AccessKeyID, config.AccessKeySecret)
	// request := dysmsapi.CreateSendBatchSmsRequest()
	// request.PhoneNumberJson = phoneNumbersJSON
	// request.SignNameJson = signNamesJSON
	// request.TemplateCode = templateCode
	// request.TemplateParamJson = templateParamsJSON
	// response, err := client.SendBatchSms(request)

	// 模拟批量发送（开发阶段）
	results := make([]*SendResponse, len(req.Tasks))
	batchID := fmt.Sprintf("mock_aliyun_batch_%s_%d", signName, time.Now().UnixNano())

	for i, task := range req.Tasks {
		results[i] = &SendResponse{
			Success:    true,
			ProviderID: fmt.Sprintf("%s_%d", batchID, i),
			TaskID:     task.TaskID,
		}
	}

	return &BatchSendResponse{Results: results}, nil
}

// ==================== CallbackHandler 接口实现 ====================

// SupportsCallback 是否支持回调
func (s *AliyunSMSSender) SupportsCallback() bool {
	return true
}

// HandleCallback 处理阿里云短信回调
// 阿里云短信状态报告格式：
// [{"phone_number":"1381111****","send_time":"2017-01-01 00:00:00","report_time":"2017-01-01 00:00:00","success":true,"err_code":"DELIVRD","err_msg":"用户接收成功","sms_size":"1","biz_id":"12345^67890","out_id":""}]
func (s *AliyunSMSSender) HandleCallback(ctx context.Context, req *CallbackRequest) ([]*CallbackResult, error) {
	// 解析回调数据
	var reports []struct {
		PhoneNumber string `json:"phone_number"`
		SendTime    string `json:"send_time"`
		ReportTime  string `json:"report_time"`
		Success     bool   `json:"success"`
		ErrCode     string `json:"err_code"`
		ErrMsg      string `json:"err_msg"`
		SmsSize     string `json:"sms_size"`
		BizId       string `json:"biz_id"`
		OutId       string `json:"out_id"`
	}

	if err := json.Unmarshal(req.RawBody, &reports); err != nil {
		return nil, fmt.Errorf("invalid callback data: %w", err)
	}

	results := make([]*CallbackResult, 0, len(reports))
	for _, report := range reports {
		status := constants.CallbackStatusDelivered
		if !report.Success {
			status = constants.CallbackStatusFailed
		}

		reportTime, _ := time.ParseInLocation("2006-01-02 15:04:05", report.ReportTime, time.Local)

		// BizId 格式可能是 "bizId^taskId" 或纯 bizId
		providerID := report.BizId
		if strings.Contains(report.BizId, "^") {
			parts := strings.Split(report.BizId, "^")
			providerID = parts[0]
		}

		results = append(results, &CallbackResult{
			ProviderID:   providerID,
			Status:       status,
			ErrorCode:    report.ErrCode,
			ErrorMessage: report.ErrMsg,
			ReportTime:   reportTime,
		})
	}

	return results, nil
}
