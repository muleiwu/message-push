package sender

import (
	"fmt"
)

// Factory 发送器工厂
type Factory struct {
	senders map[string]Sender // key: providerCode
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
	factory.Register(NewZrwinfoSMSSender())
	factory.Register(NewWeChatWorkSender())
	factory.Register(NewDingTalkSender())

	return factory
}

// Register 注册发送器
func (f *Factory) Register(sender Sender) {
	f.senders[sender.GetProviderCode()] = sender
}

// GetSender 根据服务商代码获取发送器
func (f *Factory) GetSender(providerCode string) (Sender, error) {
	sender, exists := f.senders[providerCode]
	if !exists {
		return nil, fmt.Errorf("unknown provider: %s", providerCode)
	}
	return sender, nil
}

// GetBatchSender 根据服务商代码获取批量发送器
func (f *Factory) GetBatchSender(providerCode string) (BatchSender, error) {
	sender, err := f.GetSender(providerCode)
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
	sender, exists := f.senders[providerCode]
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
	for _, sender := range f.senders {
		if handler, ok := sender.(CallbackHandler); ok && handler.SupportsCallback() {
			handlers = append(handlers, handler)
		}
	}
	return handlers
}

// GetStatusQuerier 根据服务商代码获取状态查询器
func (f *Factory) GetStatusQuerier(providerCode string) (StatusQuerier, error) {
	sender, exists := f.senders[providerCode]
	if !exists {
		return nil, fmt.Errorf("unknown provider code: %s", providerCode)
	}

	querier, ok := sender.(StatusQuerier)
	if !ok {
		return nil, fmt.Errorf("provider %s does not implement StatusQuerier", providerCode)
	}

	if !querier.SupportsStatusQuery() {
		return nil, fmt.Errorf("provider %s does not support status query", providerCode)
	}

	return querier, nil
}

// GetStatusPuller 根据服务商代码获取状态拉取器
func (f *Factory) GetStatusPuller(providerCode string) (StatusPuller, error) {
	sender, exists := f.senders[providerCode]
	if !exists {
		return nil, fmt.Errorf("unknown provider code: %s", providerCode)
	}

	puller, ok := sender.(StatusPuller)
	if !ok {
		return nil, fmt.Errorf("provider %s does not implement StatusPuller", providerCode)
	}

	if !puller.SupportsStatusPull() {
		return nil, fmt.Errorf("provider %s does not support status pull", providerCode)
	}

	return puller, nil
}

// GetAllStatusQueriers 获取所有支持状态查询的查询器
func (f *Factory) GetAllStatusQueriers() []StatusQuerier {
	var queriers []StatusQuerier
	for _, sender := range f.senders {
		if querier, ok := sender.(StatusQuerier); ok && querier.SupportsStatusQuery() {
			queriers = append(queriers, querier)
		}
	}
	return queriers
}

// GetAllStatusPullers 获取所有支持状态拉取的拉取器
func (f *Factory) GetAllStatusPullers() []StatusPuller {
	var pullers []StatusPuller
	for _, sender := range f.senders {
		if puller, ok := sender.(StatusPuller); ok && puller.SupportsStatusPull() {
			pullers = append(pullers, puller)
		}
	}
	return pullers
}
