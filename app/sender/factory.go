package sender

import (
	"fmt"

	"cnb.cool/mliev/push/message-push/app/constants"
)

// Factory 发送器工厂
type Factory struct {
	senders map[string]Sender
}

// NewFactory 创建工厂
func NewFactory() *Factory {
	factory := &Factory{
		senders: make(map[string]Sender),
	}

	// 注册所有发送器
	factory.Register(NewAliyunSMSSender())
	factory.Register(NewSMTPSender())
	factory.Register(NewTencentSMSSender())
	factory.Register(NewWeChatWorkSender())
	factory.Register(NewDingTalkSender())

	return factory
}

// Register 注册发送器
func (f *Factory) Register(sender Sender) {
	f.senders[sender.GetType()] = sender
}

// GetSender 根据消息类型获取发送器
func (f *Factory) GetSender(messageType string) (Sender, error) {
	sender, exists := f.senders[messageType]
	if !exists {
		return nil, fmt.Errorf("unsupported message type: %s", messageType)
	}
	return sender, nil
}

// GetSenderByProvider 根据服务商代码获取发送器
func (f *Factory) GetSenderByProvider(providerCode string) (Sender, error) {
	// 服务商代码映射到消息类型
	var messageType string
	switch providerCode {
	case constants.ProviderAliyunSMS, constants.ProviderTencentSMS:
		messageType = constants.MessageTypeSMS
	case constants.ProviderSMTP:
		messageType = constants.MessageTypeEmail
	case constants.ProviderWeChatWork:
		messageType = constants.MessageTypeWeChatWork
	case constants.ProviderDingTalk:
		messageType = constants.MessageTypeDingTalk
	default:
		return nil, fmt.Errorf("unknown provider code: %s", providerCode)
	}

	return f.GetSender(messageType)
}
