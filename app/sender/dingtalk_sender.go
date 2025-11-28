package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/registry"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

func init() {
	// 注册钉钉服务商
	registry.Register(&registry.ProviderMeta{
		Code:        constants.ProviderDingTalk,
		Name:        "钉钉",
		Type:        constants.MessageTypeDingTalk,
		Description: "钉钉工作通知消息服务，支持文本和Markdown消息",
		ConfigFields: []registry.ConfigField{
			{
				Key:         "app_key",
				Label:       "应用AppKey",
				Description: "钉钉应用的AppKey",
				Type:        registry.FieldTypeText,
				Required:    true,
				Example:     "dingxxxxxxxxxxxxxx",
				Placeholder: "请输入AppKey",
				HelpLink:    "https://open.dingtalk.com/document/orgapp/obtain-the-access_token-of-an-internal-app",
			},
			{
				Key:         "app_secret",
				Label:       "应用AppSecret",
				Description: "钉钉应用的AppSecret",
				Type:        registry.FieldTypePassword,
				Required:    true,
				Example:     "xxxxxxxxxxxxxxxxxxxxxx",
				Placeholder: "请输入AppSecret",
				HelpLink:    "https://open.dingtalk.com/document/orgapp/obtain-the-access_token-of-an-internal-app",
			},
			{
				Key:         "agent_id",
				Label:       "应用AgentId",
				Description: "钉钉应用的AgentId",
				Type:        registry.FieldTypeText,
				Required:    true,
				Example:     "123456789",
				Placeholder: "请输入AgentId",
			},
		},
		// 能力声明
		SupportsSend:      true,
		SupportsBatchSend: true,
		SupportsCallback:  true,
		// 扩展信息
		Website:    "https://www.dingtalk.com/",
		Icon:       "https://www.dingtalk.com/favicon.ico",
		DocsUrl:    "https://open.dingtalk.com/document/orgapp/asynchronous-sending-of-enterprise-session-messages",
		ConsoleUrl: "https://open-dev.dingtalk.com/",
		PricingUrl: "",
		SortOrder:  40,
		Tags:       []string{"企业", "即时通讯"},
		Regions:    []string{"中国大陆"},
		Deprecated: false,
	})
}

type DingTalkSender struct {
}

func NewDingTalkSender() *DingTalkSender {
	return &DingTalkSender{}
}

func (s *DingTalkSender) GetProviderCode() string {
	return constants.ProviderDingTalk
}

func (s *DingTalkSender) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	appKey, _ := config["app_key"].(string)
	appSecret, _ := config["app_secret"].(string)
	agentID, _ := config["agent_id"].(string)

	if appKey == "" || appSecret == "" || agentID == "" {
		return nil, fmt.Errorf("missing dingtalk config: app_key, app_secret or agent_id")
	}

	// 1. 获取 Access Token
	token, err := s.getAccessToken(ctx, appKey, appSecret)
	if err != nil {
		return nil, err
	}

	// 2. 构造消息
	// 支持 text, markdown
	msgType := "text"
	// 这里简单起见，默认text

	// 接收者 user_id_list
	receiver := req.Task.Receiver // userId1,userId2...

	// 使用 Content 字段，如果为空则尝试使用 Title
	content := req.Task.Content
	if content == "" {
		content = req.Task.Title
	}

	payload := map[string]interface{}{
		"agent_id":    agentID, // API might assume int
		"userid_list": receiver,
		"msg": map[string]interface{}{
			"msgtype": msgType,
			"text": map[string]string{
				"content": content,
			},
		},
	}

	// 如果是markdown...

	body, _ := json.Marshal(payload)

	// 3. 发送请求
	apiURL := fmt.Sprintf("https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2?access_token=%s", token)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var respData struct {
		ErrCode   int64  `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		TaskID    int64  `json:"task_id"`
		RequestID string `json:"request_id"`
	}

	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if respData.ErrCode != 0 {
		return &SendResponse{
			Success:      false,
			ErrorCode:    fmt.Sprintf("%d", respData.ErrCode),
			ErrorMessage: respData.ErrMsg,
			TaskID:       req.Task.TaskID,
		}, nil
	}

	return &SendResponse{
		Success:    true,
		ProviderID: fmt.Sprintf("%d", respData.TaskID),
		TaskID:     req.Task.TaskID,
		Status:     constants.TaskStatusSuccess, // 钉钉消息发送成功即完成
	}, nil
}

func (s *DingTalkSender) getAccessToken(ctx context.Context, appKey, appSecret string) (string, error) {
	redisClient := helper.GetHelper().GetRedis()
	key := fmt.Sprintf("dingtalk:token:%s", appKey)

	// 尝试从Redis获取
	token, err := redisClient.Get(ctx, key).Result()
	if err == nil && token != "" {
		return token, nil
	}

	// 从API获取
	apiURL := fmt.Sprintf("https://oapi.dingtalk.com/gettoken?appkey=%s&appsecret=%s", appKey, appSecret)
	resp, err := http.Get(apiURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	if data.ErrCode != 0 {
		return "", fmt.Errorf("failed to get access token: %s", data.ErrMsg)
	}

	// 缓存到Redis
	redisClient.Set(ctx, key, data.AccessToken, time.Duration(data.ExpiresIn-200)*time.Second)

	return data.AccessToken, nil
}

// ==================== BatchSender 接口实现 ====================

// SupportsBatchSend 是否支持批量发送
func (s *DingTalkSender) SupportsBatchSend() bool {
	return true
}

// BatchSend 批量发送钉钉消息（钉钉支持 userid_list 用逗号分隔多个用户）
func (s *DingTalkSender) BatchSend(ctx context.Context, req *BatchSendRequest) (*BatchSendResponse, error) {
	if len(req.Tasks) == 0 {
		return &BatchSendResponse{Results: []*SendResponse{}}, nil
	}

	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	appKey, _ := config["app_key"].(string)
	appSecret, _ := config["app_secret"].(string)
	agentID, _ := config["agent_id"].(string)

	if appKey == "" || appSecret == "" || agentID == "" {
		return nil, fmt.Errorf("missing dingtalk config: app_key, app_secret or agent_id")
	}

	// 获取 Access Token
	token, err := s.getAccessToken(ctx, appKey, appSecret)
	if err != nil {
		return nil, err
	}

	// 收集所有用户ID（用逗号分隔）
	var userIDs []string
	for _, task := range req.Tasks {
		if task.Receiver != "" {
			userIDs = append(userIDs, task.Receiver)
		}
	}
	userIDList := strings.Join(userIDs, ",")

	// 构造消息（批量发送时使用第一个任务的内容）
	firstTask := req.Tasks[0]
	msgType := "text"

	content := firstTask.Content
	if content == "" {
		content = firstTask.Title
	}

	payload := map[string]interface{}{
		"agent_id":    agentID,
		"userid_list": userIDList,
		"msg": map[string]interface{}{
			"msgtype": msgType,
			"text": map[string]string{
				"content": content,
			},
		},
	}

	body, _ := json.Marshal(payload)

	// 发送请求
	apiURL := fmt.Sprintf("https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2?access_token=%s", token)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var respData struct {
		ErrCode   int64  `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		TaskID    int64  `json:"task_id"`
		RequestID string `json:"request_id"`
	}

	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// 构造结果
	results := make([]*SendResponse, len(req.Tasks))
	providerID := fmt.Sprintf("%d", respData.TaskID)

	for i, task := range req.Tasks {
		if respData.ErrCode != 0 {
			results[i] = &SendResponse{
				Success:      false,
				ErrorCode:    fmt.Sprintf("%d", respData.ErrCode),
				ErrorMessage: respData.ErrMsg,
				TaskID:       task.TaskID,
			}
		} else {
			results[i] = &SendResponse{
				Success:    true,
				ProviderID: providerID,
				TaskID:     task.TaskID,
				Status:     constants.TaskStatusSuccess, // 钉钉消息发送成功即完成
			}
		}
	}

	return &BatchSendResponse{Results: results}, nil
}

// ==================== CallbackHandler 接口实现 ====================

// SupportsCallback 是否支持回调
func (s *DingTalkSender) SupportsCallback() bool {
	return true
}

// HandleCallback 处理钉钉回调
// 钉钉工作通知消息发送结果回调
func (s *DingTalkSender) HandleCallback(ctx context.Context, req *CallbackRequest) (CallbackResponse, []*CallbackResult, error) {
	// 默认响应（钉钉期望返回 "success"）
	resp := CallbackResponse{
		StatusCode: 200,
		Body:       "success",
	}

	// 钉钉回调数据格式（需要在钉钉开放平台配置回调URL）
	var callbackData struct {
		EventType string `json:"EventType"` // bpms_task_change, check_url, etc.
		TaskID    int64  `json:"task_id"`
		CorpID    string `json:"corpid"`
		UserID    string `json:"userid"`
		Status    string `json:"status"` // 0:未读, 1:已读, 2:已使用
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := json.Unmarshal(req.RawBody, &callbackData); err != nil {
		// 即使解析失败也返回成功响应，避免服务商重复推送
		return resp, nil, fmt.Errorf("invalid callback data: %w", err)
	}

	// 钉钉的消息状态：0-未读, 1-已读, 2-已使用
	status := constants.CallbackStatusDelivered
	if callbackData.ErrCode != 0 {
		status = constants.CallbackStatusFailed
	}

	reportTime := time.Unix(callbackData.Timestamp/1000, 0)
	if callbackData.Timestamp == 0 {
		reportTime = time.Now()
	}

	return resp, []*CallbackResult{{
		ProviderID:   fmt.Sprintf("%d", callbackData.TaskID),
		Status:       status,
		ErrorCode:    fmt.Sprintf("%d", callbackData.ErrCode),
		ErrorMessage: callbackData.ErrMsg,
		ReportTime:   reportTime,
	}}, nil
}
