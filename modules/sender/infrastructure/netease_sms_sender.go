package infrastructure

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	domain "cnb.cool/mliev/push/message-push/modules/sender/domain"
)

// 网易云信 API 端点（声明为 var 以便测试时重定向到本地 httptest 服务）
var (
	neteaseSendTemplateURL = "https://api.netease.im/sms/sendtemplate.action"
	neteaseSendCodeURL     = "https://api.netease.im/sms/sendcode.action"
)

// maxBatchSizeNeteaseSMS 单次模板短信最多支持的手机号数量
const maxBatchSizeNeteaseSMS = 100

// 发送类型：模板短信 / 验证码短信
const (
	neteaseSendTypeTemplate = "template"
	neteaseSendTypeCode     = "code"
)

func init() {
	// 注册网易云信短信服务商
	domain.Register(&domain.ProviderMeta{
		Code:        constants.ProviderNeteaseSMS,
		Name:        "网易云信短信",
		Type:        constants.MessageTypeSMS,
		Description: "网易云信短信服务，支持通知短信与验证码短信发送、回执抄送。注意：短信签名已内嵌在已审核的模板内容中，无需在「签名管理」中单独配置",
		ConfigFields: []domain.ConfigField{
			{
				Key:         "app_key",
				Label:       "AppKey",
				Description: "网易云信控制台分配的 AppKey",
				Type:        domain.FieldTypeText,
				Required:    true,
				Example:     "9b2a9ade419055031a6e3fab8f89e4xx",
				Placeholder: "请输入 AppKey",
				HelpLink:    "https://doc.yunxin.163.com/sms/server-apis/jg2NDEyMzI?platform=server",
			},
			{
				Key:         "app_secret",
				Label:       "AppSecret",
				Description: "网易云信控制台分配的 AppSecret，可刷新",
				Type:        domain.FieldTypePassword,
				Required:    true,
				Example:     "xxxxxxxxxxxx",
				Placeholder: "请输入 AppSecret",
				HelpLink:    "https://doc.yunxin.163.com/sms/server-apis/jg2NDEyMzI?platform=server",
			},
			{
				Key:          "send_type",
				Label:        "发送类型",
				Description:  "选择短信类型：通知短信走 sendtemplate.action；验证码短信走 sendcode.action（验证码类模板必须选「验证码短信」，否则会报 template id not exist）",
				Type:         domain.FieldTypeSelect,
				Required:     false,
				DefaultValue: neteaseSendTypeTemplate,
				Options: []domain.FieldOption{
					{Value: neteaseSendTypeTemplate, Label: "通知短信"},
					{Value: neteaseSendTypeCode, Label: "验证码短信"},
				},
			},
		},
		// 能力声明
		SupportsSend:      true,
		SupportsBatchSend: true,
		SupportsCallback:  true,
		// 扩展信息
		Website: "https://yunxin.163.com",
		// 使用仓库内置的通用图标，避免服务商列表请求不存在的静态文件。
		Icon:       "/image/logo/logo.svg",
		DocsUrl:    "https://doc.yunxin.163.com/sms/server-apis/jg2NDEyMzI?platform=server",
		ConsoleUrl: "https://app.yunxin.163.com/",
		SortOrder:  30,
		Tags:       []string{"国内"},
		Regions:    []string{"中国大陆"},
		Deprecated: false,
	})
}

// NeteaseSMSSender 网易云信短信发送器
type NeteaseSMSSender struct {
	client *http.Client
}

// NewNeteaseSMSSender 创建网易云信短信发送器
func NewNeteaseSMSSender() *NeteaseSMSSender {
	return &NeteaseSMSSender{
		client: &http.Client{
			Timeout: time.Duration(domain.DefaultTimeout) * time.Second,
		},
	}
}

// GetProviderCode 获取服务商代码
func (s *NeteaseSMSSender) GetProviderCode() string {
	return constants.ProviderNeteaseSMS
}

// genNonce 生成随机数 Nonce（32 个十六进制字符）
func (s *NeteaseSMSSender) genNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 退化兜底：使用纳秒时间戳，正常情况下不会触发
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

// buildAuthHeaders 构造网易云信鉴权请求头
// CheckSum = SHA1(AppSecret + Nonce + CurTime)，小写十六进制
func (s *NeteaseSMSSender) buildAuthHeaders(appKey, appSecret string) map[string]string {
	nonce := s.genNonce()
	curTime := strconv.FormatInt(time.Now().Unix(), 10)
	sum := sha1.Sum([]byte(appSecret + nonce + curTime))
	checkSum := hex.EncodeToString(sum[:])

	return map[string]string{
		"AppKey":       appKey,
		"Nonce":        nonce,
		"CurTime":      curTime,
		"CheckSum":     checkSum,
		"Content-Type": "application/x-www-form-urlencoded;charset=utf-8",
	}
}

// extractAppConfig 解析并校验配置，返回 appKey、appSecret 与发送类型
func (s *NeteaseSMSSender) extractAppConfig(account interface {
	GetConfig() (map[string]interface{}, error)
}) (appKey, appSecret, sendType string, err error) {
	config, err := account.GetConfig()
	if err != nil {
		return "", "", "", fmt.Errorf("invalid provider config: %w", err)
	}

	appKey, _ = config["app_key"].(string)
	appSecret, _ = config["app_secret"].(string)
	sendType, _ = config["send_type"].(string)
	if sendType == "" {
		sendType = neteaseSendTypeTemplate
	}

	if appKey == "" || appSecret == "" {
		return "", "", "", fmt.Errorf("missing netease sms config: app_key or app_secret")
	}
	return appKey, appSecret, sendType, nil
}

// resolveTemplateCode 从绑定/任务中解析模板编号与模板内容
func resolveNeteaseTemplate(binding *model.ChannelTemplateBinding, fallbackCode string) (templateCode, templateContent string) {
	if binding != nil && binding.ProviderTemplate != nil {
		templateCode = binding.ProviderTemplate.TemplateCode
		templateContent = binding.ProviderTemplate.TemplateContent
	}
	if templateCode == "" {
		templateCode = fallbackCode
	}
	return templateCode, templateContent
}

// formatNeteaseMobile 按网易云信规则格式化目标手机号：
// 中国大陆号码传裸 11 位（不带 +86 前缀）；非大陆号码需带国家/地区代码且用连字符分隔，
// 如美国 +1-xxxxxxxxxx、香港 +852-xxxxxxxx，否则网易返回 414 "mobile 'xxx' bad format!"。
// region/countryCode/nationalNumber 为 worker 预解析结果，缺失时兜底自行解析；
// 解析失败时原样返回，交由网易侧校验报错。
func formatNeteaseMobile(region, countryCode, nationalNumber, raw string) string {
	if region == "" || countryCode == "" || nationalNumber == "" {
		info := helper.ParsePhoneNumber(raw)
		if !info.Valid {
			return raw
		}
		region, countryCode, nationalNumber = info.Region, info.CountryCode, info.NationalNumber
	}
	if region == "CN" {
		return nationalNumber
	}
	return fmt.Sprintf("+%s-%s", countryCode, nationalNumber)
}

// buildParamsFromMapping 从模板内容解析占位符顺序，按序从映射参数取值，返回有序字符串切片
// 用于模板短信（sendtemplate.action）的 params 数组参数
func (s *NeteaseSMSSender) buildParamsFromMapping(templateContent string, params map[string]string) []string {
	if len(params) == 0 {
		return nil
	}

	// 没有模板内容时，按 map 顺序返回所有值
	if templateContent == "" {
		values := make([]string, 0, len(params))
		for _, v := range params {
			values = append(values, v)
		}
		return values
	}

	// 从模板内容中按出现顺序提取占位符 {name}
	re := regexp.MustCompile(`\{(\w+)\}`)
	matches := re.FindAllStringSubmatch(templateContent, -1)
	if len(matches) == 0 {
		return nil
	}

	values := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		values = append(values, params[match[1]]) // 缺失则为空字符串
	}
	return values
}

// postForm 发送表单请求并解析网易标准响应 {code,msg,obj}
// 返回：obj、响应码、响应消息(msg)、原始响应体、错误。
// 注意两个接口的字段含义相反，sendid 取处由调用方决定：
//   - sendtemplate.action：obj = sendid，msg = 发送条数
//   - sendcode.action：msg = sendid，obj = 验证码
func (s *NeteaseSMSSender) postForm(ctx context.Context, endpoint, appKey, appSecret string, form url.Values) (obj, respCode, respMsg string, body []byte, err error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to create request: %w", err)
	}
	for k, v := range s.buildAuthHeaders(appKey, appSecret) {
		httpReq.Header.Set(k, v)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return "", "", "", nil, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 响应：{"code":200,"msg":"...","obj":...}，msg/obj 可能为数字或字符串，宽松解析
	var result struct {
		Code int               `json:"code"`
		Msg  neteaseFlexString `json:"msg"`
		Obj  neteaseFlexString `json:"obj"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", "", body, fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	respCode = strconv.Itoa(result.Code)
	respMsg = string(result.Msg)
	if result.Code != 200 {
		return "", respCode, respMsg, body, nil
	}
	return string(result.Obj), respCode, respMsg, body, nil
}

// buildSendResponse 根据 postForm 的结果构造统一的发送响应
func buildSendResponse(taskID, requestData, sendID, respCode, respMsg string, body []byte, err error) *domain.SendResponse {
	if err != nil {
		return &domain.SendResponse{
			Success:      false,
			ErrorMessage: err.Error(),
			TaskID:       taskID,
			RequestData:  requestData,
			ResponseData: string(body),
		}
	}
	if sendID != "" {
		return &domain.SendResponse{
			Success:      true,
			ProviderID:   sendID,
			TaskID:       taskID,
			Status:       constants.TaskStatusSent, // 已发送，等待回执抄送
			RequestData:  requestData,
			ResponseData: string(body),
		}
	}
	return &domain.SendResponse{
		Success:      false,
		ErrorCode:    respCode,
		ErrorMessage: respMsg,
		TaskID:       taskID,
		RequestData:  requestData,
		ResponseData: string(body),
	}
}

// Send 发送短信（单发）
func (s *NeteaseSMSSender) Send(ctx context.Context, req *domain.SendRequest) (*domain.SendResponse, error) {
	// 1. 获取配置
	appKey, appSecret, sendType, err := s.extractAppConfig(req.ProviderAccount)
	if err != nil {
		return nil, err
	}

	// 2. 获取模板编号与内容（网易签名内嵌于模板，忽略 req.Signature）
	templateCode, templateContent := resolveNeteaseTemplate(req.ChannelTemplateBinding, req.Task.TemplateCode)
	if templateCode == "" {
		return nil, fmt.Errorf("missing template_code")
	}

	// 3. 按网易规则格式化手机号（大陆裸号码 / 非大陆 +国家码-号码）
	mobile := formatNeteaseMobile(req.PhoneRegion, req.PhoneCountryCode, req.PhoneNationalNumber, req.Task.Receiver)

	// 4. 按发送类型分流
	if sendType == neteaseSendTypeCode {
		return s.sendCodeOne(ctx, appKey, appSecret, templateCode, mobile, req.Task, req.MappedParams), nil
	}
	return s.sendTemplateOne(ctx, appKey, appSecret, templateCode, templateContent, mobile, req.Task, req.MappedParams), nil
}

// sendTemplateOne 通过 sendtemplate.action 发送单条模板短信
func (s *NeteaseSMSSender) sendTemplateOne(ctx context.Context, appKey, appSecret, templateCode, templateContent, mobile string, task *model.PushTask, mappedParams map[string]string) *domain.SendResponse {
	mobiles := []string{mobile}
	params := s.buildParamsFromMapping(templateContent, mappedParams)

	form := s.buildTemplateForm(templateCode, mobiles, params)
	requestData, _ := json.Marshal(map[string]interface{}{
		"templateid": templateCode,
		"mobiles":    mobiles,
		"params":     params,
	})

	sendID, respCode, respMsg, body, err := s.postForm(ctx, neteaseSendTemplateURL, appKey, appSecret, form)
	return buildSendResponse(task.TaskID, string(requestData), sendID, respCode, respMsg, body, err)
}

// sendCodeOne 通过 sendcode.action 发送单条验证码短信
// 验证码接口为单手机号，paramMap 为「变量名->值」的 JSON 对象，直接复用映射后的参数
func (s *NeteaseSMSSender) sendCodeOne(ctx context.Context, appKey, appSecret, templateCode, mobile string, task *model.PushTask, mappedParams map[string]string) *domain.SendResponse {
	form := url.Values{}
	form.Set("mobile", mobile)
	form.Set("templateid", templateCode)
	if len(mappedParams) > 0 {
		paramMapJSON, _ := json.Marshal(mappedParams)
		form.Set("paramMap", string(paramMapJSON))
	}

	requestData, _ := json.Marshal(map[string]interface{}{
		"templateid": templateCode,
		"mobile":     mobile,
		"paramMap":   mappedParams,
	})

	// sendcode.action 的 sendid 在 msg 字段（obj 为验证码，不能当 sendid 用，否则回执关联不上）
	_, respCode, respMsg, body, err := s.postForm(ctx, neteaseSendCodeURL, appKey, appSecret, form)
	sendID := ""
	if err == nil && respCode == "200" {
		sendID = respMsg
	}
	return buildSendResponse(task.TaskID, string(requestData), sendID, respCode, respMsg, body, err)
}

// buildTemplateForm 构造模板短信表单参数
func (s *NeteaseSMSSender) buildTemplateForm(templateCode string, mobiles, params []string) url.Values {
	mobilesJSON, _ := json.Marshal(mobiles)

	form := url.Values{}
	form.Set("templateid", templateCode)
	form.Set("mobiles", string(mobilesJSON))
	if len(params) > 0 {
		paramsJSON, _ := json.Marshal(params)
		form.Set("params", string(paramsJSON))
	}
	return form
}

// ==================== BatchSender 接口实现 ====================

// SupportsBatchSend 是否支持批量发送
func (s *NeteaseSMSSender) SupportsBatchSend() bool {
	return true
}

// BatchSend 批量发送短信（同模板、同参数）
func (s *NeteaseSMSSender) BatchSend(ctx context.Context, req *domain.BatchSendRequest) (*domain.BatchSendResponse, error) {
	if len(req.Tasks) == 0 {
		return &domain.BatchSendResponse{Results: []*domain.SendResponse{}}, nil
	}

	// 1. 获取配置
	appKey, appSecret, sendType, err := s.extractAppConfig(req.ProviderAccount)
	if err != nil {
		return nil, err
	}

	// 2. 获取模板编号与内容
	templateCode, templateContent := resolveNeteaseTemplate(req.ChannelTemplateBinding, req.Tasks[0].TemplateCode)
	if templateCode == "" {
		return nil, fmt.Errorf("missing template_code")
	}

	// 3. 验证码短信为单手机号接口，逐条发送
	// 批量请求未携带预解析的手机号信息，由 formatNeteaseMobile 兜底解析
	if sendType == neteaseSendTypeCode {
		results := make([]*domain.SendResponse, len(req.Tasks))
		for i, task := range req.Tasks {
			mobile := formatNeteaseMobile("", "", "", task.Receiver)
			results[i] = s.sendCodeOne(ctx, appKey, appSecret, templateCode, mobile, task, req.MappedParams)
		}
		return &domain.BatchSendResponse{Results: results}, nil
	}

	// 4. 模板短信支持 mobiles 数组批量，网易单次最多 100 个手机号，超出需分批
	params := s.buildParamsFromMapping(templateContent, req.MappedParams)
	results := make([]*domain.SendResponse, 0, len(req.Tasks))
	for start := 0; start < len(req.Tasks); start += maxBatchSizeNeteaseSMS {
		end := start + maxBatchSizeNeteaseSMS
		if end > len(req.Tasks) {
			end = len(req.Tasks)
		}
		chunk := req.Tasks[start:end]
		results = append(results, s.batchSendChunk(ctx, appKey, appSecret, templateCode, params, chunk)...)
	}

	return &domain.BatchSendResponse{Results: results}, nil
}

// batchSendChunk 发送一个不超过 100 个手机号的模板短信批次
func (s *NeteaseSMSSender) batchSendChunk(ctx context.Context, appKey, appSecret, templateCode string, params []string, tasks []*model.PushTask) []*domain.SendResponse {
	mobiles := make([]string, len(tasks))
	for i, task := range tasks {
		mobiles[i] = formatNeteaseMobile("", "", "", task.Receiver)
	}

	form := s.buildTemplateForm(templateCode, mobiles, params)
	requestData, _ := json.Marshal(map[string]interface{}{
		"templateid": templateCode,
		"mobiles":    mobiles,
		"params":     params,
	})

	sendID, respCode, respMsg, body, err := s.postForm(ctx, neteaseSendTemplateURL, appKey, appSecret, form)

	results := make([]*domain.SendResponse, len(tasks))
	for i, task := range tasks {
		switch {
		case err != nil:
			results[i] = &domain.SendResponse{
				Success:      false,
				ErrorMessage: err.Error(),
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: string(body),
			}
		case sendID != "":
			results[i] = &domain.SendResponse{
				Success: true,
				// 整批共用同一个真实 sendid（provider_msg_id 非唯一索引，允许重复）。
				// 回执抄送携带原始 sendid + mobile，由回调服务按 (provider_msg_id + 接收方手机号) 关联到具体任务。
				ProviderID:   sendID,
				TaskID:       task.TaskID,
				Status:       constants.TaskStatusSent,
				RequestData:  string(requestData),
				ResponseData: string(body),
			}
		default:
			results[i] = &domain.SendResponse{
				Success:      false,
				ErrorCode:    respCode,
				ErrorMessage: respMsg,
				TaskID:       task.TaskID,
				RequestData:  string(requestData),
				ResponseData: string(body),
			}
		}
	}
	return results
}

// ==================== CallbackHandler 接口实现 ====================

// neteaseFlexString 兼容字符串和数字两种 JSON 形式。
// 网易文档中 eventType 为字符串（"11"），实际回调下发的是数字（11），两种都需接受。
type neteaseFlexString string

func (f *neteaseFlexString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = neteaseFlexString(s)
		return nil
	}
	*f = neteaseFlexString(data)
	return nil
}

// SupportsCallback 是否支持回调
func (s *NeteaseSMSSender) SupportsCallback() bool {
	return true
}

// HandleCallback 处理网易云信短信回执抄送
// 请求方式：POST，Content-Type: application/json
// 请求头：CurTime、MD5(请求体 md5)、CheckSum(SHA1(AppSecret + MD5 + CurTime))
// 注意：HandleCallback 不接收 ProviderAccount，无法取得 AppSecret 做强校验，
//
//	因此与其它服务商一致，仅按 body 解析处理（宽松策略），优先保证有效回执不被误丢。
//
// 下行回执 body（eventType=11）：
//
//	{"eventType":"11","objects":[{"mobile":"...","sendid":"1490","result":"DELIVRD",
//	 "sendTime":"...","reportTime":"...","spliced":"1","templateId":1234,"reason":"..."}]}
//
// 上行短信 body（eventType=12，用户回复）：
//
//	{"eventType":"12","objects":[{"mobile":"...","content":"用户回复内容","receiveTime":"..."}]}
//
// 其它 eventType 暂不处理。
func (s *NeteaseSMSSender) HandleCallback(ctx context.Context, req *domain.CallbackRequest) (domain.CallbackResponse, []*domain.CallbackResult, error) {
	// 网易仅要求返回 HTTP 200
	resp := domain.CallbackResponse{
		StatusCode: 200,
		Body:       `{"code":200,"msg":"success"}`,
	}

	var payload struct {
		EventType neteaseFlexString `json:"eventType"`
		Objects   []struct {
			Mobile      neteaseFlexString `json:"mobile"`
			Sendid      neteaseFlexString `json:"sendid"`
			Result      string            `json:"result"`
			SendTime    string            `json:"sendTime"`
			ReportTime  string            `json:"reportTime"`
			Reason      string            `json:"reason"`
			Content     string            `json:"content"`     // 上行：用户回复内容
			ReceiveTime string            `json:"receiveTime"` // 上行：用户回复时间
		} `json:"objects"`
	}

	if err := json.Unmarshal(req.RawBody, &payload); err != nil {
		return resp, nil, fmt.Errorf("failed to parse netease callback: %w, body: %s", err, string(req.RawBody))
	}

	switch string(payload.EventType) {
	case "11":
		// 下行投递回执
		results := make([]*domain.CallbackResult, 0, len(payload.Objects))
		for _, obj := range payload.Objects {
			if obj.Sendid == "" {
				continue
			}
			status := constants.CallbackStatusDelivered
			if obj.Result != "DELIVRD" {
				status = constants.CallbackStatusFailed
			}
			reportTime, _ := time.ParseInLocation("2006-01-02 15:04:05", obj.ReportTime, time.Local)

			results = append(results, &domain.CallbackResult{
				Type:         constants.CallbackTypeReport,
				ProviderID:   string(obj.Sendid),
				Status:       status,
				ErrorCode:    obj.Result,
				ErrorMessage: obj.Reason,
				ReportTime:   reportTime,
				Mobile:       string(obj.Mobile), // 批量回执按 (sendid + mobile) 关联到具体任务
			})
		}
		return resp, results, nil

	case "12":
		// 上行短信（用户回复）
		results := make([]*domain.CallbackResult, 0, len(payload.Objects))
		for _, obj := range payload.Objects {
			if obj.Mobile == "" {
				continue
			}
			receiveTime, _ := time.ParseInLocation("2006-01-02 15:04:05", obj.ReceiveTime, time.Local)
			results = append(results, &domain.CallbackResult{
				Type:        constants.CallbackTypeUpstream,
				Mobile:      string(obj.Mobile),
				Content:     obj.Content,
				ReceiveTime: receiveTime,
			})
		}
		return resp, results, nil

	default:
		// 其它事件类型暂不处理
		return resp, nil, nil
	}
}
