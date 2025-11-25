package registry

import (
	"fmt"
	"sync"
)

// ProviderMeta 服务商元信息
type ProviderMeta struct {
	Code         string        `json:"code"`          // 服务商代码（唯一标识）
	Name         string        `json:"name"`          // 服务商名称
	Type         string        `json:"type"`          // 消息类型：sms, email, wechat_work, dingtalk, webhook, push
	Description  string        `json:"description"`   // 服务商描述
	ConfigFields []ConfigField `json:"config_fields"` // 配置参数定义

	// 能力声明
	SupportsSend      bool `json:"supports_send"`       // 是否支持单条发送
	SupportsBatchSend bool `json:"supports_batch_send"` // 是否支持批量发送
	SupportsCallback  bool `json:"supports_callback"`   // 是否支持回调

	// 扩展信息
	Website    string   `json:"website"`     // 官网地址
	Icon       string   `json:"icon"`        // 服务商图标URL
	DocsUrl    string   `json:"docs_url"`    // API文档链接
	ConsoleUrl string   `json:"console_url"` // 管理控制台链接
	PricingUrl string   `json:"pricing_url"` // 定价页面链接
	SortOrder  int      `json:"sort_order"`  // 排序权重（数字越小越靠前）
	Tags       []string `json:"tags"`        // 标签列表
	Regions    []string `json:"regions"`     // 支持区域
	Deprecated bool     `json:"deprecated"`  // 是否已弃用
}

// ProviderRegistry 服务商注册表
type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]*ProviderMeta // key: provider code
}

var (
	globalRegistry *ProviderRegistry
	once           sync.Once
)

// GetRegistry 获取全局注册表实例（单例）
func GetRegistry() *ProviderRegistry {
	once.Do(func() {
		globalRegistry = &ProviderRegistry{
			providers: make(map[string]*ProviderMeta),
		}
	})
	return globalRegistry
}

// Register 注册服务商
func (r *ProviderRegistry) Register(meta *ProviderMeta) error {
	if meta == nil {
		return fmt.Errorf("provider meta cannot be nil")
	}
	if meta.Code == "" {
		return fmt.Errorf("provider code cannot be empty")
	}
	if meta.Name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}
	if meta.Type == "" {
		return fmt.Errorf("provider type cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[meta.Code]; exists {
		return fmt.Errorf("provider with code %s already registered", meta.Code)
	}

	r.providers[meta.Code] = meta
	return nil
}

// GetAll 获取所有注册的服务商
func (r *ProviderRegistry) GetAll() []*ProviderMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ProviderMeta, 0, len(r.providers))
	for _, meta := range r.providers {
		result = append(result, meta)
	}
	return result
}

// GetByCode 根据代码获取服务商
func (r *ProviderRegistry) GetByCode(code string) (*ProviderMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	meta, exists := r.providers[code]
	if !exists {
		return nil, fmt.Errorf("provider with code %s not found", code)
	}
	return meta, nil
}

// GetByType 根据类型获取服务商列表
func (r *ProviderRegistry) GetByType(msgType string) []*ProviderMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ProviderMeta, 0)
	for _, meta := range r.providers {
		if meta.Type == msgType {
			result = append(result, meta)
		}
	}
	return result
}

// Exists 检查服务商是否已注册
func (r *ProviderRegistry) Exists(code string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.providers[code]
	return exists
}

// Register 全局注册函数（便捷方法）
func Register(meta *ProviderMeta) error {
	return GetRegistry().Register(meta)
}

// GetAll 全局获取所有服务商（便捷方法）
func GetAll() []*ProviderMeta {
	return GetRegistry().GetAll()
}

// GetByCode 全局根据代码获取服务商（便捷方法）
func GetByCode(code string) (*ProviderMeta, error) {
	return GetRegistry().GetByCode(code)
}

// GetByType 全局根据类型获取服务商（便捷方法）
func GetByType(msgType string) []*ProviderMeta {
	return GetRegistry().GetByType(msgType)
}
