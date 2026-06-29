package infrastructure

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
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"

	"github.com/nyaruka/phonenumbers"
)

// 掌榕网 API 端点
const (
	zrwinfoSingleSendURL   = "http://api.1cloudsp.com/api/v2/single_send"
	zrwinfoBatchSendURL    = "http://api.1cloudsp.com/api/v2/send"
	zrwinfoReportStatusURL = "http://api.1cloudsp.com/report/status"
)

func init() {
	// 注册掌榕网短信服务商
	domain.Register(&domain.ProviderMeta{
		Code:        constants.ProviderZrwinfoSMS,
		Name:        "掌榕网短信",
		Type:        constants.MessageTypeSMS,
		Description: "掌榕网融合通信产品，提供国内短信、语音、5G智慧短信等服务。注意：当前仅支持国内短信发送，接收者必须为中国大陆手机号；短信签名需在「签名管理」中单独配置",
		ConfigFields: []domain.ConfigField{
			{
				Key:         "accesskey",
				Label:       "AccessKey",
				Description: "平台分配的 accesskey，登录系统首页可点击「我的秘钥」查看",
				Type:        domain.FieldTypeText,
				Required:    true,
				Example:     "your_accesskey",
				Placeholder: "请输入 AccessKey",
			},
			{
				Key:         "secret",
				Label:       "Secret",
				Description: "平台分配的 secret，登录系统首页可点击「我的秘钥」查看",
				Type:        domain.FieldTypePassword,
				Required:    true,
				Example:     "your_secret",
				Placeholder: "请输入 Secret",
			},
		},
		// 能力声明
		SupportsSend:       true,
		SupportsBatchSend:  true,
		SupportsCallback:   true,
		SupportsStatusPull: true,
		// 扩展信息
		Website:    "https://www.zrwinfo.com",
		Icon:       "http://e.cryun.com/static/favicon.ico",
		DocsUrl:    "http://e.cryun.com/static/index.html#/home/developer/interface/info/1",
		ConsoleUrl: "http://e.cryun.com/",
		SortOrder:  20,
		Tags:       []string{"国内"},
		Regions:    []string{"中国大陆"},
		Deprecated: false,
	})
}

// ZrwinfoSMSSender 掌榕网短信发送器
type ZrwinfoSMSSender struct {
	client         *http.Client
	callbackLogDao *dao.CallbackLogDAO
}

// NewZrwinfoSMSSender 创建掌榕网短信发送器
func NewZrwinfoSMSSender() *ZrwinfoSMSSender {
	return &ZrwinfoSMSSender{
		client: &http.Client{
			Timeout: time.Duration(domain.DefaultTimeout) * time.Second,
		},
		callbackLogDao: dao.NewCallbackLogDAO(),
	}
}

// 自定义错误码：接收者不是有效的国内手机号
const zrwinfoErrCodeInvalidReceiver = "INVALID_RECEIVER"

// normalizeCNMobile 解析并校验中国大陆手机号，返回裸 11 位号码。
// 掌榕网仅支持国内短信，不需要国际区号；非国内/无效号码返回 error。
// 支持入参格式：纯 11 位、+86、0086、86 前缀。
func (s *ZrwinfoSMSSender) normalizeCNMobile(receiver string) (string, error) {
	receiver = strings.TrimSpace(receiver)
	if receiver == "" {
		return "", fmt.Errorf("phone number cannot be empty")
	}

	num, err := phonenumbers.Parse(receiver, "CN")
	if err != nil {
		return "", fmt.Errorf("非国内手机号: %s (%v)", receiver, err)
	}

	if !phonenumbers.IsValidNumber(num) || phonenumbers.GetRegionCodeForNumber(num) != "CN" {
		return "", fmt.Errorf("非国内手机号: %s", receiver)
	}

	switch phonenumbers.GetNumberType(num) {
	case phonenumbers.MOBILE, phonenumbers.FIXED_LINE_OR_MOBILE:
		// ok
	default:
		return "", fmt.Errorf("非国内手机号: %s", receiver)
	}

	return fmt.Sprintf("%d", num.GetNationalNumber()), nil
}

// resolveCNMobile 解析单发请求的手机号，优先使用 worker 预解析的地区信息（按地区直接判断），
// 缺失时兜底自行解析。返回裸 11 位国内号码；非国内/无效号码返回 error。
func (s *ZrwinfoSMSSender) resolveCNMobile(req *domain.SendRequest) (string, error) {
	if req.PhoneRegion != "" {
		if req.PhoneRegion != "CN" {
			return "", fmt.Errorf("非国内手机号: %s", req.Task.Receiver)
		}
		if req.PhoneNationalNumber != "" {
			return req.PhoneNationalNumber, nil
		}
	}
	// 兜底：未预解析（如直接调用方未填充字段）时自行解析
	return s.normalizeCNMobile(req.Task.Receiver)
}

// recordInvalidReceiver 把一条非国内号码的发送失败写入 callback_logs，便于观测。
func (s *ZrwinfoSMSSender) recordInvalidReceiver(task *model.PushTask, errMsg string) {
	if task == nil {
		return
	}
	_ = s.callbackLogDao.Create(&model.CallbackLog{
		Type:           constants.CallbackTypeReport,
		TaskID:         task.TaskID,
		AppID:          task.AppID,
		ProviderCode:   constants.ProviderZrwinfoSMS,
		Mobile:         task.Receiver,
		CallbackStatus: constants.CallbackStatusFailed,
		ErrorCode:      zrwinfoErrCodeInvalidReceiver,
		ErrorMessage:   errMsg,
	})
}

// GetProviderCode 获取服务商代码
func (s *ZrwinfoSMSSender) GetProviderCode() string {
	return constants.ProviderZrwinfoSMS
}

// Send 发送短信（单发）
func (s *ZrwinfoSMSSender) Send(ctx context.Context, req *domain.SendRequest) (*domain.SendResponse, error) {
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

	// 3. 校验并归一化手机号（掌榕网仅支持国内手机号，按地区直接判断）
	mobile, err := s.resolveCNMobile(req)
	if err != nil {
		s.recordInvalidReceiver(req.Task, err.Error())
		return &domain.SendResponse{
			Success:      false,
			ErrorCode:    zrwinfoErrCodeInvalidReceiver,
			ErrorMessage: err.Error(),
			TaskID:       req.Task.TaskID,
			RequestData:  "{}",
			ResponseData: "",
		}, nil
	}

	// 4. 转换模板参数
	content := s.buildContentFromMapping(templateContent, req.MappedParams)

	// 5. 构造请求参数
	params := url.Values{}
	params.Set("accesskey", accesskey)
	params.Set("secret", secret)
	params.Set("sign", signName)
	params.Set("templateId", templateCode)
	params.Set("mobile", mobile)
	params.Set("content", content)

	// 6. 序列化请求数据用于日志（不记录敏感信息）
	requestData, _ := json.Marshal(map[string]interface{}{
		"sign":        signName,
		"template_id": templateCode,
		"mobile":      mobile,
		"content":     content,
	})

	// 7. 发送请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", zrwinfoSingleSendURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return &domain.SendResponse{
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
		return &domain.SendResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to read response: %v", err),
			TaskID:       req.Task.TaskID,
			RequestData:  string(requestData),
			ResponseData: "",
		}, nil
	}

	// 8. 解析响应
	var result struct {
		Code   string `json:"code"`
		Msg    string `json:"msg"`
		SmUuid string `json:"smUuid"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return &domain.SendResponse{
			Success:      false,
			ErrorMessage: fmt.Sprintf("failed to parse response: %v, body: %s", err, string(body)),
			TaskID:       req.Task.TaskID,
			RequestData:  string(requestData),
			ResponseData: string(body),
		}, nil
	}

	if result.Code == "0" {
		return &domain.SendResponse{
			Success:      true,
			ProviderID:   result.SmUuid,
			TaskID:       req.Task.TaskID,
			Status:       constants.TaskStatusSent, // 已发送，等待回调
			RequestData:  string(requestData),
			ResponseData: string(body),
		}, nil
	}

	return &domain.SendResponse{
		Success:      false,
		ErrorCode:    result.Code,
		ErrorMessage: result.Msg,
		TaskID:       req.Task.TaskID,
		RequestData:  string(requestData),
		ResponseData: string(body),
	}, nil
}

// buildContentFromMapping 从映射后的参数构建内容
// 从模板内容解析占位符顺序，然后按顺序从 MappedParamsMap 中取值，用 ## 拼接
func (s *ZrwinfoSMSSender) buildContentFromMapping(templateContent string, params map[string]string) string {
	if len(params) == 0 {
		return ""
	}

	// 如果没有模板内容，直接按 map 顺序拼接
	if templateContent == "" {
		var values []string
		for _, v := range params {
			values = append(values, v)
		}
		return strings.Join(values, "##")
	}

	// 从模板内容中提取占位符顺序
	re := regexp.MustCompile(`\{(\w+)\}`)
	matches := re.FindAllStringSubmatch(templateContent, -1)

	if len(matches) == 0 {
		return ""
	}

	// 按占位符出现顺序提取参数值
	var values []string
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		key := match[1]
		if v, ok := params[key]; ok {
			values = append(values, v)
		} else {
			values = append(values, "")
		}
	}

	return strings.Join(values, "##")
}

// ==================== BatchSender 接口实现 ====================

// SupportsBatchSend 是否支持批量发送
func (s *ZrwinfoSMSSender) SupportsBatchSend() bool {
	return true
}

// BatchSend 批量发送短信
func (s *ZrwinfoSMSSender) BatchSend(ctx context.Context, req *domain.BatchSendRequest) (*domain.BatchSendResponse, error) {
	if len(req.Tasks) == 0 {
		return &domain.BatchSendResponse{Results: []*domain.SendResponse{}}, nil
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

	// 3. 批量发送使用 MappedParams（所有任务共用相同参数）
	return s.batchSendSameContent(ctx, req, accesskey, secret, signName, templateCode, templateContent)
}

// batchSendSameContent 批量发送相同内容的短信
func (s *ZrwinfoSMSSender) batchSendSameContent(ctx context.Context, req *domain.BatchSendRequest, accesskey, secret, signName, templateCode, templateContent string) (*domain.BatchSendResponse, error) {
	// 结果与 req.Tasks 一一对应
	results := make([]*domain.SendResponse, len(req.Tasks))

	// 校验并归一化手机号：有效的进入批量请求，非国内号码标记失败并落库 callback_logs
	var validIdx []int
	var validMobiles []string
	for i, task := range req.Tasks {
		mobile, err := s.normalizeCNMobile(task.Receiver)
		if err != nil {
			s.recordInvalidReceiver(task, err.Error())
			results[i] = &domain.SendResponse{
				Success:      false,
				ErrorCode:    zrwinfoErrCodeInvalidReceiver,
				ErrorMessage: err.Error(),
				TaskID:       task.TaskID,
				RequestData:  "{}",
				ResponseData: "",
			}
			continue
		}
		validIdx = append(validIdx, i)
		validMobiles = append(validMobiles, mobile)
	}

	// 全部为非国内号码，无需发送
	if len(validMobiles) == 0 {
		return &domain.BatchSendResponse{Results: results}, nil
	}

	// 转换模板参数
	content := s.buildContentFromMapping(templateContent, req.MappedParams)

	// 构造请求参数
	params := url.Values{}
	params.Set("accesskey", accesskey)
	params.Set("secret", secret)
	params.Set("sign", signName)
	params.Set("templateId", templateCode)
	params.Set("mobile", strings.Join(validMobiles, ","))
	params.Set("content", content)

	// 序列化请求数据用于日志
	requestData, _ := json.Marshal(map[string]interface{}{
		"sign":        signName,
		"template_id": templateCode,
		"mobiles":     validMobiles,
		"content":     content,
	})

	// fillValid 为所有有效号码填充相同的结果
	fillValid := func(fn func(idx int) *domain.SendResponse) *domain.BatchSendResponse {
		for _, idx := range validIdx {
			results[idx] = fn(idx)
		}
		return &domain.BatchSendResponse{Results: results}
	}

	// 发送请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", zrwinfoBatchSendURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return fillValid(func(idx int) *domain.SendResponse {
			return &domain.SendResponse{
				Success:      false,
				ErrorMessage: err.Error(),
				TaskID:       req.Tasks[idx].TaskID,
				RequestData:  string(requestData),
				ResponseData: "",
			}
		}), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fillValid(func(idx int) *domain.SendResponse {
			return &domain.SendResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("failed to read response: %v", err),
				TaskID:       req.Tasks[idx].TaskID,
				RequestData:  string(requestData),
				ResponseData: "",
			}
		}), nil
	}

	// 解析响应
	var result struct {
		Code    string `json:"code"`
		Msg     string `json:"msg"`
		BatchId string `json:"batchId"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fillValid(func(idx int) *domain.SendResponse {
			return &domain.SendResponse{
				Success:      false,
				ErrorMessage: fmt.Sprintf("failed to parse response: %v, body: %s", err, string(body)),
				TaskID:       req.Tasks[idx].TaskID,
				RequestData:  string(requestData),
				ResponseData: string(body),
			}
		}), nil
	}

	isSuccess := result.Code == "0"
	return fillValid(func(idx int) *domain.SendResponse {
		if isSuccess {
			return &domain.SendResponse{
				Success:      true,
				ProviderID:   fmt.Sprintf("%s_%d", result.BatchId, idx), // 为每条记录生成唯一标识
				TaskID:       req.Tasks[idx].TaskID,
				Status:       constants.TaskStatusSent, // 已发送，等待回调
				RequestData:  string(requestData),
				ResponseData: string(body),
			}
		}
		return &domain.SendResponse{
			Success:      false,
			ErrorCode:    result.Code,
			ErrorMessage: result.Msg,
			TaskID:       req.Tasks[idx].TaskID,
			RequestData:  string(requestData),
			ResponseData: string(body),
		}
	}), nil
}

// ==================== StatusPuller 接口实现 ====================

// SupportsStatusPull 是否支持状态拉取
func (s *ZrwinfoSMSSender) SupportsStatusPull() bool {
	return true
}

// PullStatus 批量拉取待处理状态
// 使用掌榕网 /report/status API
// 注意：已拉取的状态不会再次返回，需在开发者工具中开启"主动获取"功能
func (s *ZrwinfoSMSSender) PullStatus(ctx context.Context, req *domain.StatusPullRequest) (*domain.StatusQueryResponse, error) {
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

	// 2. 构造请求参数
	params := url.Values{}
	params.Set("accesskey", accesskey)
	params.Set("secret", secret)

	// 3. 发送请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", zrwinfoReportStatusURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 4. 解析响应
	var result struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
		Data []struct {
			SmUuid        string `json:"smUuid"`
			DeliverTime   string `json:"deliverTime"`
			Mobile        string `json:"mobile"`
			DeliverResult string `json:"deliverResult"`
			BatchId       string `json:"batchId"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	if result.Code != "0" {
		return nil, fmt.Errorf("pull status failed: code=%s, msg=%s", result.Code, result.Msg)
	}

	// 5. 转换结果
	results := make([]*domain.StatusQueryResult, 0, len(result.Data))
	for _, item := range result.Data {
		status := constants.CallbackStatusDelivered
		if item.DeliverResult != "DELIVRD" {
			status = constants.CallbackStatusFailed
		}

		reportTime, _ := time.ParseInLocation("2006-01-02 15:04:05", item.DeliverTime, time.Local)

		results = append(results, &domain.StatusQueryResult{
			ProviderMsgID: item.SmUuid,
			PhoneNumber:   item.Mobile,
			Status:        status,
			ErrorCode:     item.DeliverResult,
			ErrorMessage:  "",
			ReportTime:    reportTime,
		})
	}

	return &domain.StatusQueryResponse{Results: results}, nil
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
func (s *ZrwinfoSMSSender) HandleCallback(ctx context.Context, req *domain.CallbackRequest) (domain.CallbackResponse, []*domain.CallbackResult, error) {
	// 默认响应（掌榕网期望返回 HTTP 200 状态码）
	resp := domain.CallbackResponse{
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

	return resp, []*domain.CallbackResult{
		{
			ProviderID:   smUuid,
			Status:       status,
			ErrorCode:    deliverResult,
			ErrorMessage: "",
			ReportTime:   reportTime,
		},
	}, nil
}
