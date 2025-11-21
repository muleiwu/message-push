package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		Description: "钉钉工作通知消息推送服务，支持文本和Markdown消息",
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
	})
}

type DingTalkSender struct {
}

func NewDingTalkSender() *DingTalkSender {
	return &DingTalkSender{}
}

func (s *DingTalkSender) GetType() string {
	return constants.MessageTypeDingTalk
}

func (s *DingTalkSender) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	config, err := req.Provider.GetConfig()
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
		}, nil
	}

	return &SendResponse{
		Success:    true,
		ProviderID: fmt.Sprintf("%d", respData.TaskID),
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
