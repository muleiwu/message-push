package infrastructure

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cnb.cool/mliev/push/message-push/app/constants"
	domain "cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func init() {
	// 注册钉钉群机器人服务商
	domain.Register(&domain.ProviderMeta{
		Code:        constants.ProviderDingTalkRobot,
		Name:        "钉钉群机器人",
		Type:        constants.MessageTypeDingTalk,
		Description: "钉钉群自定义机器人 webhook，配置 webhook 地址即可向群内推送文本/Markdown 消息，支持加签与 @ 提醒",
		ConfigFields: []domain.ConfigField{
			{
				Key:         "webhook_url",
				Label:       "Webhook地址",
				Description: "群机器人的 webhook 完整地址（包含 access_token 参数）",
				Type:        domain.FieldTypeURL,
				Required:    true,
				Example:     "https://oapi.dingtalk.com/robot/send?access_token=xxxxxxxx",
				Placeholder: "请输入群机器人 webhook 地址",
				HelpLink:    "https://open.dingtalk.com/document/orgapp/custom-robot-access",
			},
			{
				Key:         "secret",
				Label:       "加签密钥",
				Description: "机器人安全设置为「加签」时填写的密钥（以 SEC 开头）；其他安全方式可留空",
				Type:        domain.FieldTypePassword,
				Required:    false,
				Example:     "SECxxxxxxxxxxxxxxxxxxxxxx",
				Placeholder: "请输入加签密钥（可选）",
			},
			{
				Key:          "msg_type",
				Label:        "消息格式",
				Description:  "发送的消息格式，text 为纯文本，markdown 为 Markdown",
				Type:         domain.FieldTypeSelect,
				Required:     false,
				DefaultValue: "text",
				Options: []domain.FieldOption{
					{Value: "text", Label: "文本"},
					{Value: "markdown", Label: "Markdown"},
				},
			},
		},
		// 能力声明（群机器人无投递状态回调，不支持批量合并）
		SupportsSend:      true,
		SupportsBatchSend: false,
		SupportsCallback:  false,
		// 扩展信息
		Website:    "https://www.dingtalk.com/",
		Icon:       "https://www.dingtalk.com/favicon.ico",
		DocsUrl:    "https://open.dingtalk.com/document/orgapp/custom-robot-access",
		ConsoleUrl: "https://open-dev.dingtalk.com/",
		PricingUrl: "",
		SortOrder:  41,
		Tags:       []string{"企业", "即时通讯", "群机器人"},
		Regions:    []string{"中国大陆"},
		Deprecated: false,
	})
}

type DingTalkRobotSender struct {
}

func NewDingTalkRobotSender() *DingTalkRobotSender {
	return &DingTalkRobotSender{}
}

func (s *DingTalkRobotSender) GetProviderCode() string {
	return constants.ProviderDingTalkRobot
}

func (s *DingTalkRobotSender) Send(ctx context.Context, req *domain.SendRequest) (*domain.SendResponse, error) {
	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	webhookURL, _ := config["webhook_url"].(string)
	if webhookURL == "" {
		return nil, fmt.Errorf("missing dingtalk robot config: webhook_url")
	}
	secret, _ := config["secret"].(string)

	msgType, _ := config["msg_type"].(string)
	if msgType != "markdown" {
		msgType = "text"
	}

	content := req.RenderedContent

	// 接收者解析为 @ 提醒：纯数字视为手机号，其余视为 userid，@all/all 提醒所有人
	atMobiles, atUserIds, isAtAll := parseDingTalkRobotAt(req.Task.Receiver)
	at := map[string]interface{}{
		"isAtAll": isAtAll,
	}
	if len(atMobiles) > 0 {
		at["atMobiles"] = atMobiles
	}
	if len(atUserIds) > 0 {
		at["atUserIds"] = atUserIds
	}

	// 构造消息体
	var payload map[string]interface{}
	if msgType == "markdown" {
		title := req.Task.Signature
		if title == "" {
			title = "通知"
		}
		payload = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": title,
				"text":  content,
			},
			"at": at,
		}
	} else {
		payload = map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": content,
			},
			"at": at,
		}
	}

	body, _ := json.Marshal(payload)

	// 加签：timestamp + sign 追加到 URL（仅在配置了 secret 时）
	apiURL := webhookURL
	if secret != "" {
		timestamp := time.Now().UnixMilli()
		sign := signDingTalkRobot(timestamp, secret)
		sep := "&"
		if !strings.Contains(apiURL, "?") {
			sep = "?"
		}
		apiURL = fmt.Sprintf("%s%stimestamp=%d&sign=%s", apiURL, sep, timestamp, url.QueryEscape(sign))
	}

	// 发送请求（使用 context）
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var respData struct {
		ErrCode int64  `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}

	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if respData.ErrCode != 0 {
		return &domain.SendResponse{
			Success:      false,
			ErrorCode:    fmt.Sprintf("%d", respData.ErrCode),
			ErrorMessage: respData.ErrMsg,
			TaskID:       req.Task.TaskID,
			RequestData:  string(body),
			ResponseData: string(respBody),
		}, nil
	}

	return &domain.SendResponse{
		Success:      true,
		TaskID:       req.Task.TaskID,
		Status:       constants.TaskStatusSuccess, // 群机器人发送成功即完成
		RequestData:  string(body),
		ResponseData: string(respBody),
	}, nil
}

// signDingTalkRobot 生成钉钉群机器人加签
// 计算方式：HMAC-SHA256(secret, timestamp + "\n" + secret) 后 base64 编码
func signDingTalkRobot(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// parseDingTalkRobotAt 解析接收者字段为钉钉 @ 提醒
// 规则：以逗号分隔；@all/all 表示提醒所有人；
// 纯数字（手机号）归入 atMobiles，其余归入 atUserIds。
func parseDingTalkRobotAt(receiver string) (atMobiles, atUserIds []string, isAtAll bool) {
	if receiver == "" {
		return nil, nil, false
	}
	for _, raw := range strings.Split(receiver, ",") {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if strings.EqualFold(item, "@all") || strings.EqualFold(item, "all") {
			isAtAll = true
			continue
		}
		if isAllDigits(item) {
			atMobiles = append(atMobiles, item)
		} else {
			atUserIds = append(atUserIds, item)
		}
	}
	return atMobiles, atUserIds, isAtAll
}
