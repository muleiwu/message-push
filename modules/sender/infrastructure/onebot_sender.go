package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func init() {
	domain.Register(&domain.ProviderMeta{
		Code:        constants.ProviderOneBot,
		Name:        "OneBot 11",
		Type:        constants.MessageTypeQQ,
		Description: "通过 OneBot 11 HTTP 服务发送 QQ 私聊和群消息，支持纯文本和 CQ 码",
		ConfigFields: []domain.ConfigField{
			{
				Key:         "base_url",
				Label:       "HTTP 服务地址",
				Type:        domain.FieldTypeURL,
				Required:    true,
				Description: "OneBot HTTP 服务根地址，可包含反向代理路径前缀，不含 API 动作或查询参数",
				Example:     "http://127.0.0.1:5700",
				Placeholder: "http://127.0.0.1:5700",
				HelpLink:    "https://github.com/botuniverse/onebot-11/blob/master/communication/http.md",
			},
			{
				Key:         "access_token",
				Label:       "Access Token",
				Type:        domain.FieldTypePassword,
				Description: "与 OneBot 服务配置一致的访问令牌；未启用鉴权时留空",
				Placeholder: "请输入访问令牌（可选）",
			},
			{
				Key:          "message_format",
				Label:        "消息格式",
				Type:         domain.FieldTypeSelect,
				DefaultValue: "text",
				Description:  "纯文本原样显示 CQ 码；CQ 码模式由 OneBot 解析图片、@ 提醒等内容",
				Options: []domain.FieldOption{
					{Value: "text", Label: "纯文本"},
					{Value: "cqcode", Label: "CQ 码"},
				},
			},
		},
		SupportsSend: true,
		Website:      "https://onebot.dev/",
		DocsUrl:      "https://github.com/botuniverse/onebot-11/blob/master/api/public.md",
		SortOrder:    50,
		Tags:         []string{"QQ", "即时通讯", "机器人"},
	})
}

type OneBotSender struct {
	client *http.Client
}

func NewOneBotSender() *OneBotSender {
	return &OneBotSender{client: &http.Client{
		Timeout: time.Duration(domain.DefaultTimeout) * time.Second,
		// A redirect must not replay a send or forward credentials elsewhere.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (s *OneBotSender) GetProviderCode() string { return constants.ProviderOneBot }

type oneBotMessage struct {
	MessageType string `json:"message_type"`
	UserID      int64  `json:"user_id,omitempty"`
	GroupID     int64  `json:"group_id,omitempty"`
	Message     string `json:"message"`
	AutoEscape  bool   `json:"auto_escape"`
}

type oneBotResponse struct {
	Status  string          `json:"status"`
	RetCode *int64          `json:"retcode"`
	Data    json.RawMessage `json:"data"`
	// These optional diagnostics are extensions used by OneBot implementations.
	Message json.RawMessage `json:"message"`
	Wording json.RawMessage `json:"wording"`
}

func (s *OneBotSender) Send(ctx context.Context, req *domain.SendRequest) (*domain.SendResponse, error) {
	result := &domain.SendResponse{Status: constants.TaskStatusFailed}
	token := ""
	fail := func(code, message string) (*domain.SendResponse, error) {
		result.ErrorCode = code
		result.ErrorMessage = redactOneBotToken(message, token)
		return result, nil
	}
	if req == nil || req.Task == nil || req.ProviderAccount == nil {
		return fail("ONEBOT_INVALID_REQUEST", "缺少任务或服务商账号")
	}
	result.TaskID = req.Task.TaskID
	target, err := helper.ParseQQReceiver(req.Task.Receiver)
	if err != nil {
		return fail("ONEBOT_INVALID_RECEIVER", err.Error())
	}
	if strings.TrimSpace(req.RenderedContent) == "" {
		return fail("ONEBOT_EMPTY_MESSAGE", "消息正文不能为空")
	}
	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return fail("ONEBOT_INVALID_CONFIG", "OneBot 账号配置不是有效的 JSON 对象")
	}
	baseURL, _ := config["base_url"].(string)
	endpoint, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" {
		return fail("ONEBOT_INVALID_CONFIG", "base_url 必须为 HTTP/HTTPS 服务地址，可包含路径前缀，不可包含用户名、密码、查询参数或片段")
	}
	if value, exists := config["access_token"]; exists {
		var ok bool
		token, ok = value.(string)
		if !ok || strings.ContainsAny(token, "\r\n") {
			return fail("ONEBOT_INVALID_CONFIG", "access_token 必须为不含换行的字符串")
		}
	}
	format := "text"
	if value, exists := config["message_format"]; exists && value != "" {
		var ok bool
		format, ok = value.(string)
		if !ok || (format != "text" && format != "cqcode") {
			return fail("ONEBOT_INVALID_CONFIG", "message_format 必须为 text 或 cqcode")
		}
	}
	payload := oneBotMessage{MessageType: target.MessageType, Message: req.RenderedContent, AutoEscape: format == "text"}
	if target.MessageType == "private" {
		payload.UserID = target.ID
	} else {
		payload.GroupID = target.ID
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fail("ONEBOT_INVALID_REQUEST", "无法编码消息正文")
	}
	result.RequestData = oneBotDiagnostic(string(body), token)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.JoinPath("send_msg").String(), bytes.NewReader(body))
	if err != nil {
		return fail("ONEBOT_INVALID_REQUEST", err.Error())
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := s.client.Do(httpReq)
	if err != nil {
		return fail("ONEBOT_HTTP_ERROR", err.Error())
	}
	defer response.Body.Close()

	const maxResponseBytes = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if len(raw) > maxResponseBytes {
		return fail("ONEBOT_INVALID_RESPONSE", "OneBot 响应超过 1 MiB")
	}
	result.ResponseData = oneBotDiagnostic(string(raw), token)
	if err != nil {
		return fail("ONEBOT_HTTP_ERROR", err.Error())
	}
	var remote oneBotResponse
	decodeErr := json.Unmarshal(raw, &remote)
	if response.StatusCode != http.StatusOK {
		return fail(fmt.Sprintf("HTTP_%d", response.StatusCode), oneBotErrorMessage(remote, response.Status))
	}
	if remote.Status == "async" || (remote.RetCode != nil && *remote.RetCode == 1) {
		return fail(constants.ErrorCodeOneBotAsyncUnsupported, "OneBot 已异步受理消息，无法确认发送结果；请使用同步发送接口。默认不重试，以免重复发送")
	}
	if decodeErr != nil {
		return fail("ONEBOT_INVALID_RESPONSE", "OneBot 响应不是有效的发送结果 JSON")
	}
	if remote.RetCode == nil {
		return fail("ONEBOT_INVALID_RESPONSE", "OneBot 响应缺少 retcode")
	}
	if remote.Status == "failed" && *remote.RetCode != 0 {
		return fail(strconv.FormatInt(*remote.RetCode, 10), oneBotErrorMessage(remote, "OneBot 发送失败，请检查 OneBot 服务日志"))
	}
	var data struct {
		MessageID *int32 `json:"message_id"`
	}
	if remote.Status != "ok" || *remote.RetCode != 0 || json.Unmarshal(remote.Data, &data) != nil || data.MessageID == nil {
		return fail("ONEBOT_INVALID_RESPONSE", "OneBot 响应状态不一致或缺少有效的 message_id")
	}
	result.Success = true
	result.Status = constants.TaskStatusSuccess
	result.ProviderID = strconv.FormatInt(int64(*data.MessageID), 10)
	return result, nil
}

func oneBotErrorMessage(remote oneBotResponse, fallback string) string {
	for _, raw := range []json.RawMessage{remote.Wording, remote.Message} {
		var message string
		if json.Unmarshal(raw, &message) == nil && message != "" {
			return message
		}
	}
	return fallback
}

// Implementations can echo authentication headers in error responses. Redact
// both literal and JSON/URL-escaped representations before persisting diagnostics.
func redactOneBotToken(value, token string) string {
	if token == "" {
		return value
	}
	encoded, _ := json.Marshal(token)
	return strings.NewReplacer(string(encoded[1:len(encoded)-1]), "[redacted]", url.QueryEscape(token), "[redacted]", token, "[redacted]").Replace(value)
}

// Push logs use JSON columns in MySQL/PostgreSQL. Preserve non-JSON upstream
// errors as a JSON string field so those failures remain inspectable there too.
func oneBotDiagnostic(value, token string) string {
	var decoded any
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	if !json.Valid([]byte(value)) || decoder.Decode(&decoded) != nil {
		decoded = map[string]any{"raw_body": value}
	}
	// Decode strings before redacting, including escaped Unicode in echoed
	// credentials. Preserve numeric IDs and response codes exactly.
	var scrub func(any) any
	scrub = func(value any) any {
		switch value := value.(type) {
		case string:
			return redactOneBotToken(value, token)
		case map[string]any:
			redacted := make(map[string]any, len(value))
			for key, item := range value {
				redacted[redactOneBotToken(key, token)] = scrub(item)
			}
			return redacted
		case []any:
			for i, item := range value {
				value[i] = scrub(item)
			}
		}
		return value
	}
	encoded, _ := json.Marshal(scrub(decoded))
	return string(encoded)
}

var _ domain.Sender = (*OneBotSender)(nil)
