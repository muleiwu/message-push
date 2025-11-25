package sender

import (
	"fmt"

	"cnb.cool/mliev/push/message-push/app/constants"
)

// Factory 发送器工厂
type Factory struct {
	senders           map[string]Sender
	sendersByProvider map[string]Sender // 按服务商代码索引
}

// NewFactory 创建工厂
func NewFactory() *Factory {
	factory := &Factory{
		senders:           make(map[string]Sender),
		sendersByProvider: make(map[string]Sender),
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

	// 如果 sender 实现了 CallbackHandler 接口，按服务商代码索引
	if handler, ok := sender.(CallbackHandler); ok {
		f.sendersByProvider[handler.GetProviderCode()] = sender
	}
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
	// 先从按服务商代码索引的 map 中查找
	if sender, exists := f.sendersByProvider[providerCode]; exists {
		return sender, nil
	}

	// 兼容旧逻辑：服务商代码映射到消息类型
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

// GetBatchSender 根据消息类型获取批量发送器
func (f *Factory) GetBatchSender(messageType string) (BatchSender, error) {
	sender, err := f.GetSender(messageType)
	if err != nil {
		return nil, err
	}

	batchSender, ok := sender.(BatchSender)
	if !ok {
		return nil, fmt.Errorf("sender for message type %s does not support batch send", messageType)
	}

	if !batchSender.SupportsBatchSend() {
		return nil, fmt.Errorf("sender for message type %s has batch send disabled", messageType)
	}

	return batchSender, nil
}

// GetBatchSenderByProvider 根据服务商代码获取批量发送器
func (f *Factory) GetBatchSenderByProvider(providerCode string) (BatchSender, error) {
	sender, err := f.GetSenderByProvider(providerCode)
	if err != nil {
		return nil, err
	}

	batchSender, ok := sender.(BatchSender)
	if !ok {
		return nil, fmt.Errorf("provider %s does not support batch send", providerCode)
	}

	if !batchSender.SupportsBatchSend() {
		return nil, fmt.Errorf("provider %s has batch send disabled", providerCode)
	}

	return batchSender, nil
}

// GetCallbackHandler 根据服务商代码获取回调处理器
func (f *Factory) GetCallbackHandler(providerCode string) (CallbackHandler, error) {
	sender, exists := f.sendersByProvider[providerCode]
	if !exists {
		return nil, fmt.Errorf("unknown provider code: %s", providerCode)
	}

	handler, ok := sender.(CallbackHandler)
	if !ok {
		return nil, fmt.Errorf("provider %s does not implement CallbackHandler", providerCode)
	}

	if !handler.SupportsCallback() {
		return nil, fmt.Errorf("provider %s does not support callback", providerCode)
	}

	return handler, nil
}

// GetAllCallbackHandlers 获取所有支持回调的处理器
func (f *Factory) GetAllCallbackHandlers() []CallbackHandler {
	var handlers []CallbackHandler
	for _, sender := range f.sendersByProvider {
		if handler, ok := sender.(CallbackHandler); ok && handler.SupportsCallback() {
			handlers = append(handlers, handler)
		}
	}
	return handlers
}
