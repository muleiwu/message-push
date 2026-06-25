package domain

// Resolver 是 sender 上下文对外暴露的领域端口：
// 按服务商代码解析发送器及其可选能力（批量、回调、状态查询/拉取）。
// 基础设施层提供实现，并通过 assembly 注册到 go-web 容器，
// 其他模块（delivery / callback / messaging）经容器以接口方式调用，实现模块解耦。
type Resolver interface {
	// GetSender 根据服务商代码获取发送器
	GetSender(providerCode string) (Sender, error)

	// GetBatchSender 根据服务商代码获取批量发送器
	GetBatchSender(providerCode string) (BatchSender, error)

	// GetCallbackHandler 根据服务商代码获取回调处理器
	GetCallbackHandler(providerCode string) (CallbackHandler, error)
	// GetAllCallbackHandlers 获取所有支持回调的处理器
	GetAllCallbackHandlers() []CallbackHandler

	// GetStatusQuerier 根据服务商代码获取状态查询器
	GetStatusQuerier(providerCode string) (StatusQuerier, error)
	// GetAllStatusQueriers 获取所有支持状态查询的查询器
	GetAllStatusQueriers() []StatusQuerier

	// GetStatusPuller 根据服务商代码获取状态拉取器
	GetStatusPuller(providerCode string) (StatusPuller, error)
	// GetAllStatusPullers 获取所有支持状态拉取的拉取器
	GetAllStatusPullers() []StatusPuller
}
