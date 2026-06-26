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
	"cnb.cool/mliev/push/message-push/app/model"
	domain "cnb.cool/mliev/push/message-push/modules/sender/domain"
)

// 网易云信 API 端点
const (
	neteaseSendTemplateURL = "https://api.netease.im/sms/sendtemplate.action"
	// maxBatchSizeNeteaseSMS 单次模板短信最多支持的手机号数量
	maxBatchSizeNeteaseSMS = 100
)

func init() {
	// 注册网易云信短信服务商
	domain.Register(&domain.ProviderMeta{
		Code:        constants.ProviderNeteaseSMS,
		Name:        "网易云信短信",
		Type:        constants.MessageTypeSMS,
		Description: "网易云信短信服务，支持模板短信发送与回执抄送。注意：短信签名已内嵌在已审核的模板内容中，无需在「签名管理」中单独配置",
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
		},
		// 能力声明
		SupportsSend:      true,
		SupportsBatchSend: true,
		SupportsCallback:  true,
		// 扩展信息
		Website:    "https://yunxin.163.com",
		Icon:       "/image/logo/netease.png",
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

// extractAppConfig 解析并校验配置
func (s *NeteaseSMSSender) extractAppConfig(account interface {
	GetConfig() (map[string]interface{}, error)
}) (appKey, appSecret string, err error) {
	config, err := account.GetConfig()
	if err != nil {
		return "", "", fmt.Errorf("invalid provider config: %w", err)
	}

	appKey, _ = config["app_key"].(string)
	appSecret, _ = config["app_secret"].(string)

	if appKey == "" || appSecret == "" {
		return "", "", fmt.Errorf("missing netease sms config: app_key or app_secret")
	}
	return appKey, appSecret, nil
}

// buildParamsFromMapping 从模板内容解析占位符顺序，按序从映射参数取值，返回有序字符串切片
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

// doSend 执行模板短信发送，返回解析后的 sendid、HTTP 响应体与错误信息
func (s *NeteaseSMSSender) doSend(ctx context.Context, appKey, appSecret, templateCode string, mobiles, params []string) (sendID string, body []byte, respCode string, respMsg string, err error) {
	mobilesJSON, _ := json.Marshal(mobiles)

	form := url.Values{}
	form.Set("templateid", templateCode)
	form.Set("mobiles", string(mobilesJSON))
	if len(params) > 0 {
		paramsJSON, _ := json.Marshal(params)
		form.Set("params", string(paramsJSON))
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", neteaseSendTemplateURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", nil, "", "", fmt.Errorf("failed to create request: %w", err)
	}
	for k, v := range s.buildAuthHeaders(appKey, appSecret) {
		httpReq.Header.Set(k, v)
	}

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return "", nil, "", "", err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, "", "", fmt.Errorf("failed to read response: %w", err)
	}

	// 解析响应：{"code":200,"msg":"...","obj":<sendid>}，obj 为数字，用 json.Number 保留精度
	var result struct {
		Code int         `json:"code"`
		Msg  string      `json:"msg"`
		Obj  json.Number `json:"obj"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&result); err != nil {
		return "", body, "", "", fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	respCode = strconv.Itoa(result.Code)
	respMsg = result.Msg
	if result.Code != 200 {
		return "", body, respCode, respMsg, nil
	}
	return result.Obj.String(), body, respCode, respMsg, nil
}

// Send 发送短信（单发）
func (s *NeteaseSMSSender) Send(ctx context.Context, req *domain.SendRequest) (*domain.SendResponse, error) {
	// 1. 获取配置
	appKey, appSecret, err := s.extractAppConfig(req.ProviderAccount)
	if err != nil {
		return nil, err
	}

	// 2. 获取模板编号与内容（网易签名内嵌于模板，忽略 req.Signature）
	templateCode := ""
	templateContent := ""
	if req.ChannelTemplateBinding != nil && req.ChannelTemplateBinding.ProviderTemplate != nil {
		templateCode = req.ChannelTemplateBinding.ProviderTemplate.TemplateCode
		templateContent = req.ChannelTemplateBinding.ProviderTemplate.TemplateContent
	}
	if templateCode == "" {
		templateCode = req.Task.TemplateCode
	}
	if templateCode == "" {
		return nil, fmt.Errorf("missing template_code")
	}

	// 3. 构造参数
	mobiles := []string{req.Task.Receiver}
	params := s.buildParamsFromMapping(templateContent, req.MappedParams)

	// 4. 请求数据用于日志（不记录 app_secret）
	requestData, _ := json.Marshal(map[string]interface{}{
		"templateid": templateCode,
		"mobiles":    mobiles,
		"params":     params,
	})

	// 5. 发送
	sendID, body, respCode, respMsg, err := s.doSend(ctx, appKey, appSecret, templateCode, mobiles, params)
	if err != nil {
		return &domain.SendResponse{
			Success:      false,
			ErrorMessage: err.Error(),
			TaskID:       req.Task.TaskID,
			RequestData:  string(requestData),
			ResponseData: string(body),
		}, nil
	}

	if sendID != "" {
		return &domain.SendResponse{
			Success:      true,
			ProviderID:   sendID,
			TaskID:       req.Task.TaskID,
			Status:       constants.TaskStatusSent, // 已发送，等待回执抄送
			RequestData:  string(requestData),
			ResponseData: string(body),
		}, nil
	}

	return &domain.SendResponse{
		Success:      false,
		ErrorCode:    respCode,
		ErrorMessage: respMsg,
		TaskID:       req.Task.TaskID,
		RequestData:  string(requestData),
		ResponseData: string(body),
	}, nil
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
	appKey, appSecret, err := s.extractAppConfig(req.ProviderAccount)
	if err != nil {
		return nil, err
	}

	// 2. 获取模板编号与内容
	templateCode := ""
	templateContent := ""
	if req.ChannelTemplateBinding != nil && req.ChannelTemplateBinding.ProviderTemplate != nil {
		templateCode = req.ChannelTemplateBinding.ProviderTemplate.TemplateCode
		templateContent = req.ChannelTemplateBinding.ProviderTemplate.TemplateContent
	}
	if templateCode == "" {
		templateCode = req.Tasks[0].TemplateCode
	}
	if templateCode == "" {
		return nil, fmt.Errorf("missing template_code")
	}

	params := s.buildParamsFromMapping(templateContent, req.MappedParams)

	results := make([]*domain.SendResponse, 0, len(req.Tasks))
	// 网易单次最多 100 个手机号，超出需分批
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

// batchSendChunk 发送一个不超过 100 个手机号的批次
func (s *NeteaseSMSSender) batchSendChunk(ctx context.Context, appKey, appSecret, templateCode string, params []string, tasks []*model.PushTask) []*domain.SendResponse {
	mobiles := make([]string, len(tasks))
	for i, task := range tasks {
		mobiles[i] = task.Receiver
	}

	requestData, _ := json.Marshal(map[string]interface{}{
		"templateid": templateCode,
		"mobiles":    mobiles,
		"params":     params,
	})

	sendID, body, respCode, respMsg, err := s.doSend(ctx, appKey, appSecret, templateCode, mobiles, params)

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
				Success:      true,
				ProviderID:   fmt.Sprintf("%s_%d", sendID, i), // 整批共用一个 sendid，为每条生成唯一标识
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
// eventType=12 为上行短信（用户回复），非投递回执，直接忽略。
func (s *NeteaseSMSSender) HandleCallback(ctx context.Context, req *domain.CallbackRequest) (domain.CallbackResponse, []*domain.CallbackResult, error) {
	// 网易仅要求返回 HTTP 200
	resp := domain.CallbackResponse{
		StatusCode: 200,
		Body:       `{"code":200,"msg":"success"}`,
	}

	var payload struct {
		EventType string `json:"eventType"`
		Objects   []struct {
			Mobile     string `json:"mobile"`
			Sendid     string `json:"sendid"`
			Result     string `json:"result"`
			SendTime   string `json:"sendTime"`
			ReportTime string `json:"reportTime"`
			Reason     string `json:"reason"`
		} `json:"objects"`
	}

	if err := json.Unmarshal(req.RawBody, &payload); err != nil {
		return resp, nil, fmt.Errorf("failed to parse netease callback: %w, body: %s", err, string(req.RawBody))
	}

	// 仅处理下行投递回执
	if payload.EventType != "11" {
		return resp, nil, nil
	}

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
			ProviderID:   obj.Sendid,
			Status:       status,
			ErrorCode:    obj.Result,
			ErrorMessage: obj.Reason,
			ReportTime:   reportTime,
		})
	}

	return resp, results, nil
}
