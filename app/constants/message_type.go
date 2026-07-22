package constants

// 消息类型常量
const (
	MessageTypeSMS        = "sms"         // 短信
	MessageTypeEmail      = "email"       // 邮件
	MessageTypeWeChatWork = "wechat_work" // 企业微信
	MessageTypeDingTalk   = "dingtalk"    // 钉钉
	MessageTypeWebhook    = "webhook"     // Webhook
	MessageTypePush       = "push"        // 推送通知
)

var supportedMessageTypes = []string{
	MessageTypeSMS,
	MessageTypeEmail,
	MessageTypeWeChatWork,
	MessageTypeDingTalk,
	MessageTypeWebhook,
	MessageTypePush,
}

// SupportedMessageTypes 返回稳定的消息类型集合。返回副本，避免调用方修改全局定义。
func SupportedMessageTypes() []string {
	return append([]string(nil), supportedMessageTypes...)
}

// 服务商代码常量
const (
	ProviderAliyunSMS  = "aliyun_sms"  // 阿里云短信
	ProviderTencentSMS = "tencent_sms" // 腾讯云短信
	ProviderZrwinfoSMS = "zrwinfo_sms" // 掌榕网短信
	ProviderNeteaseSMS = "netease_sms" // 网易云信短信
	ProviderSMTP       = "smtp"        // SMTP邮件
	ProviderWeChatWork = "wechat_work" // 企业微信（应用消息）
	ProviderDingTalk   = "dingtalk"    // 钉钉（工作通知）

	ProviderWeChatWorkRobot = "wechat_work_robot" // 企业微信群机器人（webhook）
	ProviderDingTalkRobot   = "dingtalk_robot"    // 钉钉群机器人（webhook）
)

// IsValidMessageType 检查消息类型是否有效
func IsValidMessageType(msgType string) bool {
	for _, supported := range supportedMessageTypes {
		if msgType == supported {
			return true
		}
	}
	return false
}

// IsValidProviderCode 检查服务商代码是否有效
func IsValidProviderCode(code string) bool {
	switch code {
	case ProviderAliyunSMS, ProviderTencentSMS, ProviderZrwinfoSMS, ProviderNeteaseSMS, ProviderSMTP, ProviderWeChatWork, ProviderDingTalk, ProviderWeChatWorkRobot, ProviderDingTalkRobot:
		return true
	default:
		return false
	}
}

// 回调状态常量
const (
	CallbackStatusDelivered = "delivered" // 已送达
	CallbackStatusFailed    = "failed"    // 发送失败
	CallbackStatusRejected  = "rejected"  // 被拒绝
	CallbackStatusTimeout   = "timeout"   // 回调超时
)

// 服务商回调类型常量
// report   = 下行投递回执（短信是否到达用户）
// upstream = 上行短信（终端用户回复的短信内容）
const (
	CallbackTypeReport   = "report"   // 下行投递回执（默认）
	CallbackTypeUpstream = "upstream" // 上行短信（用户回复）
)
