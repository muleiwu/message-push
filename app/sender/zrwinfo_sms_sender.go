package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/registry"
)

// 掌榕网 API 端点
const (
	zrwinfoSingleSendURL = "http://api.1cloudsp.com/api/v2/single_send"
	zrwinfoBatchSendURL  = "http://api.1cloudsp.com/api/v2/send"
)

func init() {
	// 注册掌榕网短信服务商
	registry.Register(&registry.ProviderMeta{
		Code:        constants.ProviderZrwinfoSMS,
		Name:        "掌榕网短信",
		Type:        constants.MessageTypeSMS,
		Description: "掌榕网融合通信产品，提供短信、国际短信、语音、5G智慧短信等服务。注意：短信签名需在「签名管理」中单独配置",
		ConfigFields: []registry.ConfigField{
			{
				Key:         "accesskey",
				Label:       "AccessKey",
				Description: "平台分配的 accesskey，登录系统首页可点击「我的秘钥」查看",
				Type:        registry.FieldTypeText,
				Required:    true,
				Example:     "your_accesskey",
				Placeholder: "请输入 AccessKey",
			},
			{
				Key:         "secret",
				Label:       "Secret",
				Description: "平台分配的 secret，登录系统首页可点击「我的秘钥」查看",
				Type:        registry.FieldTypePassword,
				Required:    true,
				Example:     "your_secret",
				Placeholder: "请输入 Secret",
			},
		},
		// 能力声明
		SupportsSend:      true,
		SupportsBatchSend: true,
		SupportsCallback:  true,
		// 扩展信息
		Website:    "https://www.zrwinfo.com",
		Icon:       "http://e.cryun.com/static/favicon.ico",
		DocsUrl:    "http://e.cryun.com/static/index.html#/home/developer/interface/info/1",
		ConsoleUrl: "http://e.cryun.com/",
		SortOrder:  20,
		Tags:       []string{"国内", "国际"},
		Regions:    []string{"中国大陆", "国际"},
		Deprecated: false,
	})
}

// ZrwinfoSMSSender 掌榕网短信发送器
type ZrwinfoSMSSender struct {
	client *http.Client
}

// NewZrwinfoSMSSender 创建掌榕网短信发送器
func NewZrwinfoSMSSender() *ZrwinfoSMSSender {
	return &ZrwinfoSMSSender{
		client: &http.Client{
			Timeout: time.Duration(DefaultTimeout) * time.Second,
		},
	}
}

// GetProviderCode 获取服务商代码
func (s *ZrwinfoSMSSender) GetProviderCode() string {
	return constants.ProviderZrwinfoSMS
}

// Send 发送短信（单发）
func (s *ZrwinfoSMSSender) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	// 1. 获取配置
	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	accesskey, _ := config["accesskey"].(string)
	secret, _ := config["secret"].(string)

	if accesskey == "" || secret == "" {
		return nil, fmt.Errorf("missing zrwinfo sms config: accesskey or secret")
	}

	// 2. 获取签名和模板
	signName := ""
	templateCode := ""
	templateContent := ""

	// 从 ChannelTemplateBinding 获取模板信息
	if req.ChannelTemplateBinding != nil && req.ChannelTemplateBinding.ProviderTemplate != nil {
		templateCode = req.ChannelTemplateBinding.ProviderTemplate.TemplateCode
		templateContent = req.ChannelTemplateBinding.ProviderTemplate.TemplateContent
	}

	// 从 Signature 获取签名
	if req.Signature != nil {
		signName = req.Signature.SignatureCode
	}

	// 确保签名格式为【xxx】
	if signName != "" && !strings.HasPrefix(signName, "【") {
		signName = "【" + signName + "】"
	}

	// 兜底：从任务获取模板代码
	if templateCode == "" {
		templateCode = req.Task.TemplateCode
	}

	if templateCode == "" {
		return nil, fmt.Errorf("missing template_code")
	}

	// 3. 转换模板参数
	content, err := s.convertTemplateParams(templateContent, req.Task.TemplateParams)
	if err != nil {
		return nil, fmt.Errorf("failed to convert template params: %w", err)
	}

	// 4. 构造请求参数
	params := url.Values{}
	params.Set("accesskey", accesskey)
	params.Set("secret", secret)
	params.Set("sign", signName)
	params.Set("templateId", templateCode)
	params.Set("mobile", req.Task.Receiver)
	params.Set("content", content)

	// 5. 序列化请求数据用于日志（不记录敏感信息）
	requestData, _ := json.Marshal(map[string]interface{}{
		"sign":        signName,
		"template_id": templateCode,
		"mobile":      req.Task.Receiver,
		"content":     content,
	})

	// 6. 发送请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", zrwinfoSingleSendURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return &SendResponse{
			Success:      false,
			ErrorMessage: err.Error(),
			TaskID:       req.Task.TaskID,
			RequestData:  string(requestData),
			ResponseData: "",
		}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &SendResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to read response: %v", err),
			TaskID:       req.Task.TaskID,
			RequestData:  string(requestData),
			ResponseData: "",
		}, nil
	}

	// 7. 解析响应
	var result struct {
		Code   string `json:"code"`
		Msg    string `json:"msg"`
		SmUuid string `json:"smUuid"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return &SendResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to parse response: %v, body: %s", err, string(body)),
			TaskID:       req.Task.TaskID,
			RequestData:  string(requestData),
			ResponseData: string(body),
		}, nil
	}

	if result.Code == "0" {
		return &SendResponse{
			Success:      true,
			ProviderID:   result.SmUuid,
			TaskID:       req.Task.TaskID,
			Status:       constants.TaskStatusSent, // 已发送，等待回调
			RequestData:  string(requestData),
			ResponseData: string(body),
		}, nil
	}

	return &SendResponse{
		Success:      false,
		ErrorCode:    result.Code,
		ErrorMessage: result.Msg,
		TaskID:       req.Task.TaskID,
		RequestData:  string(requestData),
		ResponseData: string(body),
	}, nil
}

// convertTemplateParams 转换模板参数
// 从模板内容解析占位符顺序，然后按顺序从参数 JSON 中取值，用 ## 拼接
// 例如：模板 "{user}您好，您的订单于{time}已发货"，参数 {"user":"张三","time":"9:40"}
// 返回 "张三##9:40"
func (s *ZrwinfoSMSSender) convertTemplateParams(templateContent, templateParams string) (string, error) {
	if templateParams == "" {
		return "", nil
	}

	// 解析参数 JSON
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(templateParams), &params); err != nil {
		return "", fmt.Errorf("invalid template params JSON: %w", err)
	}

	if len(params) == 0 {
		return "", nil
	}

	// 如果没有模板内容，按参数 key 的字母顺序拼接（兜底方案）
	if templateContent == "" {
		var values []string
		for _, v := range params {
			values = append(values, fmt.Sprintf("%v", v))
		}
		return strings.Join(values, "##"), nil
	}

	// 从模板内容中提取占位符顺序
	// 支持 {var} 格式的占位符
	re := regexp.MustCompile(`\{(\w+)\}`)
	matches := re.FindAllStringSubmatch(templateContent, -1)

	if len(matches) == 0 {
		// 没有找到占位符，返回空
		return "", nil
	}

	// 按占位符出现顺序提取参数值
	var values []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		key := match[1]
		if v, ok := params[key]; ok {
			values = append(values, fmt.Sprintf("%v", v))
		} else {
			// 占位符在参数中不存在，使用空字符串
			values = append(values, "")
		}
	}

	return strings.Join(values, "##"), nil
}

// ==================== BatchSender 接口实现 ====================

// SupportsBatchSend 是否支持批量发送
func (s *ZrwinfoSMSSender) SupportsBatchSend() bool {
	return true
}

// BatchSend 批量发送短信
func (s *ZrwinfoSMSSender) BatchSend(ctx context.Context, req *BatchSendRequest) (*BatchSendResponse, error) {
	if len(req.Tasks) == 0 {
		return &BatchSendResponse{Results: []*SendResponse{}}, nil
	}

	// 1. 获取配置
	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	accesskey, _ := config["accesskey"].(string)
	secret, _ := config["secret"].(string)

	if accesskey == "" || secret == "" {
		return nil, fmt.Errorf("missing zrwinfo sms config: accesskey or secret")
	}

	// 2. 获取签名和模板
	signName := ""
	templateCode := ""
	templateContent := ""

	// 从 ChannelTemplateBinding 获取模板信息
	if req.ChannelTemplateBinding != nil && req.ChannelTemplateBinding.ProviderTemplate != nil {
		templateCode = req.ChannelTemplateBinding.ProviderTemplate.TemplateCode
		templateContent = req.ChannelTemplateBinding.ProviderTemplate.TemplateContent
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

	// 3. 检查是否所有任务的模板参数相同（使用 mobile+content 批量发送）
	// 或者使用 data 字段发送个性化短信
	allSameParams := true
	firstParams := req.Tasks[0].TemplateParams
	for _, task := range req.Tasks[1:] {
		if task.TemplateParams != firstParams {
			allSameParams = false
			break
		}
	}

	if allSameParams {
		// 所有参数相同，使用 mobile + content 方式
		return s.batchSendSameContent(ctx, req, accesskey, secret, signName, templateCode, templateContent)
	}

	// 参数不同，使用 data 字段发送个性化短信
	return s.batchSendPersonalized(ctx, req, accesskey, secret, signName, templateCode, templateContent)
}

// batchSendSameContent 批量发送相同内容的短信
func (s *ZrwinfoSMSSender) batchSendSameContent(ctx context.Context, req *BatchSendRequest, accesskey, secret, signName, templateCode, templateContent string) (*BatchSendResponse, error) {
	// 收集所有手机号
	mobiles := make([]string, len(req.Tasks))
	for i, task := range req.Tasks {
		mobiles[i] = task.Receiver
	}

	// 转换模板参数（使用第一个任务的参数）
	content, err := s.convertTemplateParams(templateContent, req.Tasks[0].TemplateParams)
	if err != nil {
		return nil, fmt.Errorf("failed to convert template params: %w", err)
	}

	// 构造请求参数
	params := url.Values{}
	params.Set("accesskey", accesskey)
	params.Set("secret", secret)
	params.Set("sign", signName)
	params.Set("templateId", templateCode)
	params.Set("mobile", strings.Join(mobiles, ","))
	params.Set("content", content)

	// 序列化请求数据用于日志
	requestData, _ := json.Marshal(map[string]interface{}{
		"sign":        signName,
		"template_id": templateCode,
		"mobiles":     mobiles,
		"content":     content,
	})

	// 发送请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", zrwinfoBatchSendURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := s.client.Do(httpReq)
	if err != nil {
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
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		results := make([]*SendResponse, len(req.Tasks))
		for i, task := range req.Tasks {
			results[i] = &SendResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("failed to read response: %v", err),
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: "",
			}
		}
		return &BatchSendResponse{Results: results}, nil
	}

	// 解析响应
	var result struct {
		Code    string `json:"code"`
		Msg     string `json:"msg"`
		BatchId string `json:"batchId"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		results := make([]*SendResponse, len(req.Tasks))
		for i, task := range req.Tasks {
			results[i] = &SendResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("failed to parse response: %v, body: %s", err, string(body)),
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: string(body),
			}
		}
		return &BatchSendResponse{Results: results}, nil
	}

	results := make([]*SendResponse, len(req.Tasks))
	isSuccess := result.Code == "0"

	for i, task := range req.Tasks {
		if isSuccess {
			results[i] = &SendResponse{
				Success:      true,
				ProviderID:   fmt.Sprintf("%s_%d", result.BatchId, i), // 为每条记录生成唯一标识
				TaskID:       task.TaskID,
				Status:       constants.TaskStatusSent, // 已发送，等待回调
				RequestData:  string(requestData),
				ResponseData: string(body),
			}
		} else {
			results[i] = &SendResponse{
				Success:      false,
				ErrorCode:    result.Code,
				ErrorMessage: result.Msg,
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: string(body),
			}
		}
	}

	return &BatchSendResponse{Results: results}, nil
}

// batchSendPersonalized 批量发送个性化内容的短信
func (s *ZrwinfoSMSSender) batchSendPersonalized(ctx context.Context, req *BatchSendRequest, accesskey, secret, signName, templateCode, templateContent string) (*BatchSendResponse, error) {
	// 构造 data 字段：{"手机号": "内容", ...}
	data := make(map[string]string)
	for _, task := range req.Tasks {
		content, err := s.convertTemplateParams(templateContent, task.TemplateParams)
		if err != nil {
			// 单个任务参数转换失败，记录错误但继续处理其他任务
			continue
		}
		data[task.Receiver] = content
	}

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	// 构造请求参数
	params := url.Values{}
	params.Set("accesskey", accesskey)
	params.Set("secret", secret)
	params.Set("sign", signName)
	params.Set("templateId", templateCode)
	params.Set("data", string(dataJSON))

	// 序列化请求数据用于日志
	requestData, _ := json.Marshal(map[string]interface{}{
		"sign":        signName,
		"template_id": templateCode,
		"data":        data,
	})

	// 发送请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", zrwinfoBatchSendURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := s.client.Do(httpReq)
	if err != nil {
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
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		results := make([]*SendResponse, len(req.Tasks))
		for i, task := range req.Tasks {
			results[i] = &SendResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("failed to read response: %v", err),
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: "",
			}
		}
		return &BatchSendResponse{Results: results}, nil
	}

	// 解析响应
	var result struct {
		Code    string `json:"code"`
		Msg     string `json:"msg"`
		BatchId string `json:"batchId"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		results := make([]*SendResponse, len(req.Tasks))
		for i, task := range req.Tasks {
			results[i] = &SendResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("failed to parse response: %v, body: %s", err, string(body)),
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: string(body),
			}
		}
		return &BatchSendResponse{Results: results}, nil
	}

	results := make([]*SendResponse, len(req.Tasks))
	isSuccess := result.Code == "0"

	for i, task := range req.Tasks {
		if isSuccess {
			results[i] = &SendResponse{
				Success:      true,
				ProviderID:   fmt.Sprintf("%s_%d", result.BatchId, i),
				TaskID:       task.TaskID,
				Status:       constants.TaskStatusSent, // 已发送，等待回调
				RequestData:  string(requestData),
				ResponseData: string(body),
			}
		} else {
			results[i] = &SendResponse{
				Success:      false,
				ErrorCode:    result.Code,
				ErrorMessage: result.Msg,
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: string(body),
			}
		}
	}

	return &BatchSendResponse{Results: results}, nil
}

// ==================== CallbackHandler 接口实现 ====================

// SupportsCallback 是否支持回调
func (s *ZrwinfoSMSSender) SupportsCallback() bool {
	return true
}

// HandleCallback 处理掌榕网短信回调
// 请求方式：POST，Content-Type: application/form-data;charset=utf-8
// 表单参数：
//   - smUuid: 短信唯一标识，例 10000_1_0_13700000000_1_NKWkEcS_1
//   - mobile: 手机号码，例 13700000000
//   - batchId: 批次id（可选），例 abc123456
//   - deliverResult: 回执状态，DELIVRD 成功，其他失败
//   - deliverTime: 状态码回执时间，格式 yyyy-MM-dd HH:mm:ss
func (s *ZrwinfoSMSSender) HandleCallback(ctx context.Context, req *CallbackRequest) (CallbackResponse, []*CallbackResult, error) {
	// 默认响应（掌榕网期望返回 HTTP 200 状态码）
	resp := CallbackResponse{
		StatusCode: 200,
		Body:       `{"code":"0","msg":"SUCCESS"}`,
	}

	// 从表单数据中读取字段
	smUuid := req.FormData["smUuid"]
	deliverResult := req.FormData["deliverResult"]
	deliverTime := req.FormData["deliverTime"]

	// 验证必填字段
	if smUuid == "" {
		return resp, nil, fmt.Errorf("missing required field: smUuid")
	}

	// 判断状态
	status := constants.CallbackStatusDelivered
	if deliverResult != "DELIVRD" {
		status = constants.CallbackStatusFailed
	}

	// 解析时间
	reportTime, _ := time.ParseInLocation("2006-01-02 15:04:05", deliverTime, time.Local)

	return resp, []*CallbackResult{
		{
			ProviderID:   smUuid,
			Status:       status,
			ErrorCode:    deliverResult,
			ErrorMessage: "",
			ReportTime:   reportTime,
		},
	}, nil
}
