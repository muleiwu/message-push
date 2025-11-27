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
		// 能力声明
		SupportsSend:      true,
		SupportsBatchSend: true,
		SupportsCallback:  true,
		// 扩展信息
		Website:    "https://work.weixin.qq.com/",
		Icon:       "/image/logo/wechat_work.png",
		DocsUrl:    "https://developer.work.weixin.qq.com/document/path/90664",
		ConsoleUrl: "https://work.weixin.qq.com/wework_admin/frame",
		PricingUrl: "",
		SortOrder:  30,
		Tags:       []string{"企业", "即时通讯"},
		Regions:    []string{"中国大陆"},
		Deprecated: false,
	})
}

type WeChatWorkSender struct {
}

func NewWeChatWorkSender() *WeChatWorkSender {
	return &WeChatWorkSender{}
}

func (s *WeChatWorkSender) GetProviderCode() string {
	return constants.ProviderWeChatWork
}

func (s *WeChatWorkSender) Send(ctx context.Context, req *SendRequest) (*SendResponse, error) {
	config, err := req.ProviderAccount.GetConfig()
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
			TaskID:       req.Task.TaskID,
		}, nil
	}

	return &SendResponse{
		Success:    true,
		ProviderID: respData.MsgID,
		TaskID:     req.Task.TaskID,
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

// ==================== BatchSender 接口实现 ====================

// SupportsBatchSend 是否支持批量发送
func (s *WeChatWorkSender) SupportsBatchSend() bool {
	return true
}

// BatchSend 批量发送企业微信消息（企业微信支持 touser 用 | 分隔多个用户）
func (s *WeChatWorkSender) BatchSend(ctx context.Context, req *BatchSendRequest) (*BatchSendResponse, error) {
	if len(req.Tasks) == 0 {
		return &BatchSendResponse{Results: []*SendResponse{}}, nil
	}

	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	corpID, _ := config["corp_id"].(string)
	agentSecret, _ := config["agent_secret"].(string)
	agentID, _ := config["agent_id"].(string)

	if corpID == "" || agentSecret == "" || agentID == "" {
		return nil, fmt.Errorf("missing wechat work config: corp_id, agent_secret or agent_id")
	}

	// 获取 Access Token
	token, err := s.getAccessToken(ctx, corpID, agentSecret)
	if err != nil {
		return nil, err
	}

	// 收集所有用户ID（用 | 分隔）
	var userIDs []string
	for _, task := range req.Tasks {
		if task.Receiver != "" {
			userIDs = append(userIDs, task.Receiver)
		}
	}
	toUser := strings.Join(userIDs, "|")

	// 构造消息（批量发送时使用第一个任务的内容）
	firstTask := req.Tasks[0]
	msgType := "text"
	if firstTask.MessageType == "markdown" {
		msgType = "markdown"
	}

	content := firstTask.Content
	if content == "" {
		content = firstTask.Title
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

	// 发送请求
	apiURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var respData struct {
		ErrCode      int    `json:"errcode"`
		ErrMsg       string `json:"errmsg"`
		MsgID        string `json:"msgid"`
		InvalidUser  string `json:"invaliduser"`
		InvalidParty string `json:"invalidparty"`
		InvalidTag   string `json:"invalidtag"`
	}

	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// 构造结果
	results := make([]*SendResponse, len(req.Tasks))
	invalidUsers := make(map[string]bool)
	if respData.InvalidUser != "" {
		for _, u := range strings.Split(respData.InvalidUser, "|") {
			invalidUsers[u] = true
		}
	}

	for i, task := range req.Tasks {
		if respData.ErrCode != 0 {
			results[i] = &SendResponse{
				Success:      false,
				ErrorCode:    fmt.Sprintf("%d", respData.ErrCode),
				ErrorMessage: respData.ErrMsg,
				TaskID:       task.TaskID,
			}
		} else if invalidUsers[task.Receiver] {
			results[i] = &SendResponse{
				Success:      false,
				ErrorCode:    "invalid_user",
				ErrorMessage: "用户ID无效",
				TaskID:       task.TaskID,
			}
		} else {
			results[i] = &SendResponse{
				Success:    true,
				ProviderID: respData.MsgID,
				TaskID:     task.TaskID,
			}
		}
	}

	return &BatchSendResponse{Results: results}, nil
}

// ==================== CallbackHandler 接口实现 ====================

// SupportsCallback 是否支持回调
func (s *WeChatWorkSender) SupportsCallback() bool {
	return true
}

// HandleCallback 处理企业微信回调
// 企业微信消息发送状态回调格式（需要在企业微信后台配置回调URL）
func (s *WeChatWorkSender) HandleCallback(ctx context.Context, req *CallbackRequest) (CallbackResponse, []*CallbackResult, error) {
	// 默认响应（企业微信期望返回 {"errcode": 0, "errmsg": "ok"}）
	resp := CallbackResponse{
		StatusCode: 200,
		Body:       `{"errcode":0,"errmsg":"ok"}`,
	}

	// 企业微信回调数据格式
	var callbackData struct {
		MsgID      string `json:"msgid"`
		Status     string `json:"status"` // SEND_OK, SEND_FAIL
		UserID     string `json:"userid"`
		ErrCode    int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
		CreateTime int64  `json:"create_time"`
	}

	if err := json.Unmarshal(req.RawBody, &callbackData); err != nil {
		// 即使解析失败也返回成功响应，避免服务商重复推送
		return resp, nil, fmt.Errorf("invalid callback data: %w", err)
	}

	status := constants.CallbackStatusDelivered
	if callbackData.Status != "SEND_OK" {
		status = constants.CallbackStatusFailed
	}

	reportTime := time.Unix(callbackData.CreateTime, 0)
	if callbackData.CreateTime == 0 {
		reportTime = time.Now()
	}

	return resp, []*CallbackResult{{
		ProviderID:   callbackData.MsgID,
		Status:       status,
		ErrorCode:    fmt.Sprintf("%d", callbackData.ErrCode),
		ErrorMessage: callbackData.ErrMsg,
		ReportTime:   reportTime,
	}}, nil
}
