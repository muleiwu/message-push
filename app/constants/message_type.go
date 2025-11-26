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

// 服务商代码常量
const (
	ProviderAliyunSMS  = "aliyun_sms"  // 阿里云短信
	ProviderTencentSMS = "tencent_sms" // 腾讯云短信
	ProviderZrwinfoSMS = "zrwinfo_sms" // 掌榕网短信
	ProviderSMTP       = "smtp"        // SMTP邮件
	ProviderWeChatWork = "wechat_work" // 企业微信
	ProviderDingTalk   = "dingtalk"    // 钉钉
)

// IsValidMessageType 检查消息类型是否有效
func IsValidMessageType(msgType string) bool {
	switch msgType {
	case MessageTypeSMS, MessageTypeEmail, MessageTypeWeChatWork, MessageTypeDingTalk, MessageTypeWebhook, MessageTypePush:
		return true
	default:
		return false
	}
}

// IsValidProviderCode 检查服务商代码是否有效
func IsValidProviderCode(code string) bool {
	switch code {
	case ProviderAliyunSMS, ProviderTencentSMS, ProviderZrwinfoSMS, ProviderSMTP, ProviderWeChatWork, ProviderDingTalk:
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
)
