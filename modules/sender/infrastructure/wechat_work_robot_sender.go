package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"cnb.cool/mliev/push/message-push/app/constants"
	domain "cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func init() {
	// 注册企业微信群机器人服务商
	domain.Register(&domain.ProviderMeta{
		Code:        constants.ProviderWeChatWorkRobot,
		Name:        "企业微信群机器人",
		Type:        constants.MessageTypeWeChatWork,
		Description: "企业微信群机器人 webhook，配置 webhook 地址即可向群内推送文本/Markdown 消息，支持 @ 提醒",
		ConfigFields: []domain.ConfigField{
			{
				Key:         "webhook_url",
				Label:       "Webhook地址",
				Description: "群机器人的 webhook 完整地址（包含 key 参数）",
				Type:        domain.FieldTypeURL,
				Required:    true,
				Example:     "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
				Placeholder: "请输入群机器人 webhook 地址",
				HelpLink:    "https://developer.work.weixin.qq.com/document/path/99110",
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
		Website:    "https://work.weixin.qq.com/",
		Icon:       "/image/logo/wechat_work.png",
		DocsUrl:    "https://developer.work.weixin.qq.com/document/path/99110",
		ConsoleUrl: "https://work.weixin.qq.com/wework_admin/frame",
		PricingUrl: "",
		SortOrder:  31,
		Tags:       []string{"企业", "即时通讯", "群机器人"},
		Regions:    []string{"中国大陆"},
		Deprecated: false,
	})
}

type WeChatWorkRobotSender struct {
}

func NewWeChatWorkRobotSender() *WeChatWorkRobotSender {
	return &WeChatWorkRobotSender{}
}

func (s *WeChatWorkRobotSender) GetProviderCode() string {
	return constants.ProviderWeChatWorkRobot
}

func (s *WeChatWorkRobotSender) Send(ctx context.Context, req *domain.SendRequest) (*domain.SendResponse, error) {
	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	webhookURL, _ := config["webhook_url"].(string)
	if webhookURL == "" {
		return nil, fmt.Errorf("missing wechat work robot config: webhook_url")
	}

	msgType, _ := config["msg_type"].(string)
	if msgType != "markdown" {
		msgType = "text"
	}

	content := req.RenderedContent

	// 构造消息体
	var payload map[string]interface{}
	if msgType == "markdown" {
		// Markdown 类型不支持 mentioned_list，如需 @ 请在内容中使用 <@userid>
		payload = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": content,
			},
		}
	} else {
		text := map[string]interface{}{
			"content": content,
		}
		// 接收者解析为 @ 提醒：纯数字视为手机号，其余视为 userid，@all 提醒所有人
		mentionedList, mentionedMobileList := parseWeChatRobotMentions(req.Task.Receiver)
		if len(mentionedList) > 0 {
			text["mentioned_list"] = mentionedList
		}
		if len(mentionedMobileList) > 0 {
			text["mentioned_mobile_list"] = mentionedMobileList
		}
		payload = map[string]interface{}{
			"msgtype": "text",
			"text":    text,
		}
	}

	body, _ := json.Marshal(payload)

	// 发送请求（使用 context）
	httpReq, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(body))
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
		ErrCode int    `json:"errcode"`
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

// parseWeChatRobotMentions 解析接收者字段为企业微信 @ 提醒列表
// 规则：以逗号分隔；@all/all 加入 userid 列表表示提醒所有人；
// 纯数字（手机号）归入 mentioned_mobile_list，其余归入 mentioned_list。
func parseWeChatRobotMentions(receiver string) (mentionedList, mentionedMobileList []string) {
	if receiver == "" {
		return nil, nil
	}
	for _, raw := range strings.Split(receiver, ",") {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if strings.EqualFold(item, "@all") || strings.EqualFold(item, "all") {
			mentionedList = append(mentionedList, "@all")
			continue
		}
		if isAllDigits(item) {
			mentionedMobileList = append(mentionedMobileList, item)
		} else {
			mentionedList = append(mentionedList, item)
		}
	}
	return mentionedList, mentionedMobileList
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
