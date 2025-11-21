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
	// 注册企业微信服务商
	registry.Register(&registry.ProviderMeta{
		Code:        constants.ProviderWeChatWork,
		Name:        "企业微信",
		Type:        constants.MessageTypeWeChatWork,
		Description: "企业微信应用消息推送服务，支持文本和Markdown消息",
		ConfigFields: []registry.ConfigField{
			{
				Key:         "corp_id",
				Label:       "企业ID",
				Description: "企业微信的企业ID（CorpID）",
				Type:        registry.FieldTypeText,
				Required:    true,
				Example:     "ww1234567890abcdef",
				Placeholder: "请输入企业ID",
				HelpLink:    "https://developer.work.weixin.qq.com/document/path/90665",
			},
			{
				Key:         "agent_secret",
				Label:       "应用Secret",
				Description: "企业微信应用的Secret",
				Type:        registry.FieldTypePassword,
				Required:    true,
				Example:     "xxxxxxxxxxxxxxxxxxxxxx",
				Placeholder: "请输入应用Secret",
				HelpLink:    "https://developer.work.weixin.qq.com/document/path/90665",
			},
			{
				Key:         "agent_id",
				Label:       "应用ID",
				Description: "企业微信应用的AgentID",
				Type:        registry.FieldTypeText,
				Required:    true,
				Example:     "1000002",
				Placeholder: "请输入应用ID",
			},
		},
	})
}

type WeChatWorkSender struct {
}

func NewWeChatWorkSender() *WeChatWorkSender {
	return &WeChatWorkSender{}
}

func (s *WeChatWorkSender) GetType() string {
	return constants.MessageTypeWeChatWork
}

func (s *WeChatWorkSender) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	config, err := req.Provider.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	corpID, _ := config["corp_id"].(string)
	agentSecret, _ := config["agent_secret"].(string)
	agentID, _ := config["agent_id"].(string)

	if corpID == "" || agentSecret == "" || agentID == "" {
		return nil, fmt.Errorf("missing wechat work config: corp_id, agent_secret or agent_id")
	}

	// 1. 获取 Access Token
	token, err := s.getAccessToken(ctx, corpID, agentSecret)
	if err != nil {
		return nil, err
	}

	// 2. 构造消息
	// 默认发送文本消息，支持markdown
	msgType := "text"
	if req.Task.MessageType == "markdown" {
		msgType = "markdown"
	}

	// 接收者：touser
	toUser := req.Task.Receiver
	if toUser == "" {
		toUser = "@all"
	}

	// 使用 Content 字段，如果为空则尝试使用 Title
	content := req.Task.Content
	if content == "" {
		content = req.Task.Title
	}

	payload := map[string]interface{}{
		"touser":  toUser,
		"msgtype": msgType,
		"agentid": agentID,
		"text": map[string]string{
			"content": content,
		},
		"safe": 0,
	}

	if msgType == "markdown" {
		payload["markdown"] = map[string]string{
			"content": content,
		}
		delete(payload, "text")
	}

	body, _ := json.Marshal(payload)

	// 3. 发送请求
	apiURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var respData struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		MsgID   string `json:"msgid"`
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
		ProviderID: respData.MsgID,
	}, nil
}

func (s *WeChatWorkSender) getAccessToken(ctx context.Context, corpID, secret string) (string, error) {
	redisClient := helper.GetHelper().GetRedis()
	key := fmt.Sprintf("wechat:token:%s:%s", corpID, secret)

	// 尝试从Redis获取
	token, err := redisClient.Get(ctx, key).Result()
	if err == nil && token != "" {
		return token, nil
	}

	// 从API获取
	apiURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", corpID, secret)
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
