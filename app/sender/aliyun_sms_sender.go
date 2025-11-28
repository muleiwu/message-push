package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/registry"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v3/client"
	"github.com/alibabacloud-go/tea/tea"
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
		SupportsSend:        true,
		SupportsBatchSend:   true,
		SupportsCallback:    true,
		SupportsStatusQuery: true,
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
	// 1. 获取配置
	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	accessKeyID, _ := config["access_key_id"].(string)
	accessKeySecret, _ := config["access_key_secret"].(string)

	if accessKeyID == "" || accessKeySecret == "" {
		return nil, fmt.Errorf("missing aliyun sms config: access_key_id or access_key_secret")
	}

	// 2. 初始化客户端
	client, err := s.createClient(accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create aliyun sms client: %w", err)
	}

	// 3. 构造请求
	// 签名和模板
	signName := ""
	templateCode := ""

	// 从 ChannelTemplateBinding 获取模板信息
	if req.ChannelTemplateBinding != nil && req.ChannelTemplateBinding.ProviderTemplate != nil {
		templateCode = req.ChannelTemplateBinding.ProviderTemplate.TemplateCode
	}

	// 从 Signature 获取签名
	if req.Signature != nil {
		signName = req.Signature.SignatureCode
	}

	// 兜底：从任务获取模板代码
	if templateCode == "" {
		templateCode = req.Task.TemplateCode
	}

	if templateCode == "" {
		return nil, fmt.Errorf("missing template_code")
	}

	// 构造阿里云短信发送请求
	sendRequest := &dysmsapi.SendSmsRequest{
		PhoneNumbers: tea.String(req.Task.Receiver),
		SignName:     tea.String(signName),
		TemplateCode: tea.String(templateCode),
	}

	// 模板参数（阿里云要求JSON对象格式）
	var templateParamStr string
	if len(req.MappedParams) > 0 {
		paramBytes, _ := json.Marshal(req.MappedParams)
		templateParamStr = string(paramBytes)
		sendRequest.TemplateParam = tea.String(templateParamStr)
	}

	// 4. 序列化请求数据用于日志
	requestData, _ := json.Marshal(map[string]interface{}{
		"phone_numbers":   req.Task.Receiver,
		"sign_name":       signName,
		"template_code":   templateCode,
		"template_params": templateParamStr,
	})

	// 5. 发送
	response, err := client.SendSms(sendRequest)
	if err != nil {
		return &SendResponse{
			Success:      false,
			ErrorMessage: err.Error(),
			TaskID:       req.Task.TaskID,
			RequestData:  string(requestData),
			ResponseData: "",
		}, nil
	}

	// 6. 序列化响应数据用于日志
	responseData, _ := json.Marshal(response.Body)

	// 7. 解析响应
	if response.Body == nil {
		return &SendResponse{
			Success:      false,
			ErrorMessage: "empty response from aliyun",
			TaskID:       req.Task.TaskID,
			RequestData:  string(requestData),
			ResponseData: "",
		}, nil
	}

	if tea.StringValue(response.Body.Code) == "OK" {
		return &SendResponse{
			Success:      true,
			ProviderID:   tea.StringValue(response.Body.BizId),
			TaskID:       req.Task.TaskID,
			Status:       constants.TaskStatusSent, // 已发送，等待回调
			RequestData:  string(requestData),
			ResponseData: string(responseData),
		}, nil
	}

	return &SendResponse{
		Success:      false,
		ErrorCode:    tea.StringValue(response.Body.Code),
		ErrorMessage: tea.StringValue(response.Body.Message),
		TaskID:       req.Task.TaskID,
		RequestData:  string(requestData),
		ResponseData: string(responseData),
	}, nil
}

// createClient 创建阿里云短信客户端
func (s *AliyunSMSSender) createClient(accessKeyID, accessKeySecret string) (*dysmsapi.Client, error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(accessKeyID),
		AccessKeySecret: tea.String(accessKeySecret),
		Endpoint:        tea.String("dysmsapi.aliyuncs.com"),
	}
	return dysmsapi.NewClient(config)
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

	// 1. 获取配置
	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	accessKeyID, _ := config["access_key_id"].(string)
	accessKeySecret, _ := config["access_key_secret"].(string)

	if accessKeyID == "" || accessKeySecret == "" {
		return nil, fmt.Errorf("missing aliyun sms config: access_key_id or access_key_secret")
	}

	// 2. 初始化客户端
	client, err := s.createClient(accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create aliyun sms client: %w", err)
	}

	// 3. 构造请求
	// 签名和模板
	signName := ""
	templateCode := ""

	// 从 ChannelTemplateBinding 获取模板信息
	if req.ChannelTemplateBinding != nil && req.ChannelTemplateBinding.ProviderTemplate != nil {
		templateCode = req.ChannelTemplateBinding.ProviderTemplate.TemplateCode
	}

	// 从 Signature 获取签名
	if req.Signature != nil {
		signName = req.Signature.SignatureCode
	}

	// 兜底：从第一个任务获取模板代码
	if templateCode == "" && len(req.Tasks) > 0 {
		templateCode = req.Tasks[0].TemplateCode
	}

	if templateCode == "" {
		return nil, fmt.Errorf("missing template_code")
	}

	// 收集所有手机号、签名和模板参数
	phoneNumbers := make([]string, len(req.Tasks))
	signNames := make([]string, len(req.Tasks))
	templateParams := make([]string, len(req.Tasks))

	// 批量发送时使用 MappedParams（所有任务共用相同参数）
	var mappedParamStr = "{}"
	if len(req.MappedParams) > 0 {
		paramBytes, _ := json.Marshal(req.MappedParams)
		mappedParamStr = string(paramBytes)
	}

	for i, task := range req.Tasks {
		phoneNumbers[i] = task.Receiver
		signNames[i] = signName // 批量发送时，每个号码需要对应一个签名
		templateParams[i] = mappedParamStr
	}

	// 序列化为JSON数组
	phoneNumbersJSON, _ := json.Marshal(phoneNumbers)
	signNamesJSON, _ := json.Marshal(signNames)
	templateParamsJSON, _ := json.Marshal(templateParams)

	// 构造批量发送请求
	batchRequest := &dysmsapi.SendBatchSmsRequest{
		PhoneNumberJson:   tea.String(string(phoneNumbersJSON)),
		SignNameJson:      tea.String(string(signNamesJSON)),
		TemplateCode:      tea.String(templateCode),
		TemplateParamJson: tea.String(string(templateParamsJSON)),
	}

	// 4. 序列化请求数据用于日志
	requestData, _ := json.Marshal(map[string]interface{}{
		"phone_numbers":   phoneNumbers,
		"sign_names":      signNames,
		"template_code":   templateCode,
		"template_params": templateParams,
	})

	// 5. 发送
	response, err := client.SendBatchSms(batchRequest)
	if err != nil {
		// 如果批量发送失败，返回所有任务都失败的结果
		results := make([]*SendResponse, len(req.Tasks))
		for i, task := range req.Tasks {
			results[i] = &SendResponse{
				Success:      false,
				ErrorMessage: err.Error(),
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: "",
			}
		}
		return &BatchSendResponse{Results: results}, nil
	}

	// 6. 序列化响应数据用于日志
	responseData, _ := json.Marshal(response.Body)

	// 7. 解析响应
	// 阿里云批量发送成功时返回一个统一的 BizId
	if response.Body == nil {
		results := make([]*SendResponse, len(req.Tasks))
		for i, task := range req.Tasks {
			results[i] = &SendResponse{
				Success:      false,
				ErrorMessage: "empty response from aliyun",
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: "",
			}
		}
		return &BatchSendResponse{Results: results}, nil
	}

	results := make([]*SendResponse, len(req.Tasks))
	isSuccess := tea.StringValue(response.Body.Code) == "OK"
	batchBizId := tea.StringValue(response.Body.BizId)
	errorCode := tea.StringValue(response.Body.Code)
	errorMessage := tea.StringValue(response.Body.Message)

	for i, task := range req.Tasks {
		if isSuccess {
			results[i] = &SendResponse{
				Success:      true,
				ProviderID:   fmt.Sprintf("%s_%d", batchBizId, i), // 为每条记录生成唯一标识
				TaskID:       task.TaskID,
				Status:       constants.TaskStatusSent, // 已发送，等待回调
				RequestData:  string(requestData),
				ResponseData: string(responseData),
			}
		} else {
			results[i] = &SendResponse{
				Success:      false,
				ErrorCode:    errorCode,
				ErrorMessage: errorMessage,
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: string(responseData),
			}
		}
	}

	return &BatchSendResponse{Results: results}, nil
}

// ==================== CallbackHandler 接口实现 ====================

// SupportsCallback 是否支持回调
func (s *AliyunSMSSender) SupportsCallback() bool {
	return true
}

// ==================== StatusQuerier 接口实现 ====================

// SupportsStatusQuery 是否支持状态查询
func (s *AliyunSMSSender) SupportsStatusQuery() bool {
	return true
}

// QueryStatus 查询短信发送状态
// 使用阿里云 QuerySendDetails API
func (s *AliyunSMSSender) QueryStatus(ctx context.Context, req *StatusQueryRequest) (*StatusQueryResponse, error) {
	// 1. 获取配置
	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	accessKeyID, _ := config["access_key_id"].(string)
	accessKeySecret, _ := config["access_key_secret"].(string)

	if accessKeyID == "" || accessKeySecret == "" {
		return nil, fmt.Errorf("missing aliyun sms config: access_key_id or access_key_secret")
	}

	// 2. 初始化客户端
	client, err := s.createClient(accessKeyID, accessKeySecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create aliyun sms client: %w", err)
	}

	// 3. 构造查询请求
	queryRequest := &dysmsapi.QuerySendDetailsRequest{
		PhoneNumber: tea.String(req.PhoneNumber),
		SendDate:    tea.String(req.SendDate.Format("20060102")),
		PageSize:    tea.Int64(10),
		CurrentPage: tea.Int64(1),
	}

	// 如果有 BizId，添加到查询条件
	if req.ProviderMsgID != "" {
		queryRequest.BizId = tea.String(req.ProviderMsgID)
	}

	// 4. 发送查询请求
	response, err := client.QuerySendDetails(queryRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to query send details: %w", err)
	}

	// 5. 解析响应
	if response.Body == nil || tea.StringValue(response.Body.Code) != "OK" {
		errCode := ""
		errMsg := "unknown error"
		if response.Body != nil {
			errCode = tea.StringValue(response.Body.Code)
			errMsg = tea.StringValue(response.Body.Message)
		}
		return nil, fmt.Errorf("query failed: code=%s, message=%s", errCode, errMsg)
	}

	// 6. 转换结果
	results := make([]*StatusQueryResult, 0)
	if response.Body.SmsSendDetailDTOs != nil && response.Body.SmsSendDetailDTOs.SmsSendDetailDTO != nil {
		for _, detail := range response.Body.SmsSendDetailDTOs.SmsSendDetailDTO {
			status := constants.CallbackStatusFailed
			sendStatus := tea.Int64Value(detail.SendStatus)
			// 阿里云状态：1-等待回执，2-发送失败，3-发送成功
			switch sendStatus {
			case 1:
				status = "pending" // 等待回执
			case 2:
				status = constants.CallbackStatusFailed
			case 3:
				status = constants.CallbackStatusDelivered
			}

			reportTime, _ := time.ParseInLocation("2006-01-02 15:04:05", tea.StringValue(detail.ReceiveDate), time.Local)

			results = append(results, &StatusQueryResult{
				ProviderMsgID: req.ProviderMsgID,
				PhoneNumber:   tea.StringValue(detail.PhoneNum),
				Status:        status,
				ErrorCode:     tea.StringValue(detail.ErrCode),
				ErrorMessage:  "",
				ReportTime:    reportTime,
			})
		}
	}

	return &StatusQueryResponse{Results: results}, nil
}

// HandleCallback 处理阿里云短信回调
// 阿里云短信状态报告格式：
// [{"phone_number":"1381111****","send_time":"2017-01-01 00:00:00","report_time":"2017-01-01 00:00:00","success":true,"err_code":"DELIVRD","err_msg":"用户接收成功","sms_size":"1","biz_id":"12345^67890","out_id":""}]
func (s *AliyunSMSSender) HandleCallback(ctx context.Context, req *CallbackRequest) (CallbackResponse, []*CallbackResult, error) {
	// 默认响应（阿里云期望返回 {"code" : 0, "msg" : "接收成功"} ）
	resp := CallbackResponse{
		StatusCode: 200,
		Body:       `{"code" : 0, "msg" : "接收成功"}`,
	}

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
		// 即使解析失败也返回成功响应，避免服务商重复推送
		return resp, nil, fmt.Errorf("invalid callback data: %w", err)
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
		//if strings.Contains(report.BizId, "^") {
		//	parts := strings.Split(report.BizId, "^")
		//	providerID = parts[0]
		//}

		results = append(results, &CallbackResult{
			ProviderID:   providerID,
			Status:       status,
			ErrorCode:    report.ErrCode,
			ErrorMessage: report.ErrMsg,
			ReportTime:   reportTime,
		})
	}

	return resp, results, nil
}
