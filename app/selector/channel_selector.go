package selector

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
)

// 缓存 key 前缀
const cacheKeyPrefix = "channel_selector:"

// ChannelNode 通道节点（带权重）
type ChannelNode struct {
	ChannelTemplateBinding *model.ChannelTemplateBinding // 通道模板绑定配置
	ProviderAccount        *model.ProviderAccount        // 服务商账号配置
	CurrentWeight          int                           // 当前权重
	EffectiveWeight        int                           // 有效权重
}

// ChannelSelector 通道选择器
type ChannelSelector struct {
	logger                    gsr.Logger
	channelTemplateBindingDao *dao.ChannelTemplateBindingDAO
	providerAccountDAO        *dao.ProviderAccountDAO
	cache                     gsr.Cacher    // 使用统一缓存接口
	cacheTTL                  time.Duration // 缓存过期时间
	weightMu                  sync.Mutex    // 保护权重修改的并发安全
}

// NewChannelSelector 创建通道选择器
func NewChannelSelector() *ChannelSelector {
	h := helper.GetHelper()
	return &ChannelSelector{
		logger:                    h.GetLogger(),
		channelTemplateBindingDao: dao.NewChannelTemplateBindingDAO(),
		providerAccountDAO:        dao.NewProviderAccountDAO(),
		cache:                     h.GetCache(),
		cacheTTL:                  30 * time.Second, // 默认30秒
	}
}

// buildCacheKey 构建缓存 key
func buildCacheKey(channelID uint, messageType string) string {
	return fmt.Sprintf("%s%d:%s", cacheKeyPrefix, channelID, messageType)
}

// Select 选择通道（平滑加权轮询）
func (s *ChannelSelector) Select(ctx context.Context, channelID uint, messageType string) (*ChannelNode, error) {
	nodes, err := s.getChannelNodes(ctx, channelID, messageType)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no available channel for channel_id=%d type=%s", channelID, messageType)
	}

	// 使用平滑加权轮询选择
	selected := s.smoothWeightedRoundRobin(nodes)
	if selected == nil {
		return nil, fmt.Errorf("failed to select channel")
	}

	return selected, nil
}

// getChannelNodes 获取通道节点列表（使用新的ChannelTemplateBinding）
func (s *ChannelSelector) getChannelNodes(ctx context.Context, channelID uint, messageType string) ([]*ChannelNode, error) {
	cacheKey := buildCacheKey(channelID, messageType)

	// 使用 GetSet 方法，自带缓存穿透保护
	var nodes []*ChannelNode
	err := s.cache.GetSet(ctx, cacheKey, s.cacheTTL, &nodes, func(key string, obj any) error {
		// 缓存未命中，从数据库加载
		loadedNodes, loadErr := s.loadChannelNodesFromDB(channelID)
		if loadErr != nil {
			return loadErr
		}

		// 将结果赋值给 obj
		nodesPtr := obj.(*[]*ChannelNode)
		*nodesPtr = loadedNodes
		return nil
	})

	if err != nil {
		return nil, err
	}

	// 过滤可用节点
	return s.filterAvailableNodes(nodes), nil
}

// loadChannelNodesFromDB 从数据库加载通道节点
func (s *ChannelSelector) loadChannelNodesFromDB(channelID uint) ([]*ChannelNode, error) {
	// 获取通道的所有模板绑定配置
	channelBindings, err := s.channelTemplateBindingDao.GetActiveByChannelID(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel template bindings: %w", err)
	}

	if len(channelBindings) == 0 {
		return nil, fmt.Errorf("no channel template bindings configured for channel_id=%d", channelID)
	}

	// 构建节点列表
	var nodes []*ChannelNode
	for _, ctb := range channelBindings {
		if ctb.ProviderTemplate == nil {
			s.logger.Warn(fmt.Sprintf("incomplete channel template binding id=%d", ctb.ID))
			continue
		}

		// 直接使用预加载的 ProviderAccount
		providerAccount := ctb.ProviderTemplate.ProviderAccount
		if providerAccount == nil {
			// 如果预加载失败，尝试手动查询
			var err error
			providerAccount, err = s.providerAccountDAO.GetByID(ctb.ProviderID)
			if err != nil {
				s.logger.Warn(fmt.Sprintf("failed to get provider account id=%d: %v", ctb.ProviderID, err))
				continue
			}
		}

		node := &ChannelNode{
			ChannelTemplateBinding: ctb,
			ProviderAccount:        providerAccount,
			CurrentWeight:          0,
			EffectiveWeight:        ctb.Weight,
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// filterAvailableNodes 过滤可用节点
// 同时检查 status（管理员手动控制）和 is_active（系统熔断控制）
func (s *ChannelSelector) filterAvailableNodes(nodes []*ChannelNode) []*ChannelNode {
	var available []*ChannelNode
	for _, node := range nodes {
		if node.ChannelTemplateBinding != nil &&
			node.ChannelTemplateBinding.Status == 1 &&
			node.ChannelTemplateBinding.IsActive == 1 {
			available = append(available, node)
		}
	}
	return available
}

// smoothWeightedRoundRobin 平滑加权轮询算法
func (s *ChannelSelector) smoothWeightedRoundRobin(nodes []*ChannelNode) *ChannelNode {
	if len(nodes) == 0 {
		return nil
	}

	if len(nodes) == 1 {
		return nodes[0]
	}

	// 按优先级分组
	priorityGroups := make(map[int][]*ChannelNode)
	minPriority := -1

	for _, node := range nodes {
		priority := 100 // 默认优先级
		if node.ChannelTemplateBinding != nil {
			priority = node.ChannelTemplateBinding.Priority
		}

		if minPriority == -1 || priority < minPriority {
			minPriority = priority
		}

		priorityGroups[priority] = append(priorityGroups[priority], node)
	}

	// 使用最高优先级组（数字最小）
	candidates := priorityGroups[minPriority]
	if len(candidates) == 0 {
		return nil
	}

	if len(candidates) == 1 {
		return candidates[0]
	}

	// 加锁保护权重修改的并发安全
	s.weightMu.Lock()
	defer s.weightMu.Unlock()

	// 在同优先级组内使用加权轮询
	var totalWeight int
	var selected *ChannelNode

	for _, node := range candidates {
		// 当前权重 += 有效权重
		node.CurrentWeight += node.EffectiveWeight
		totalWeight += node.EffectiveWeight

		// 选择当前权重最大的节点
		if selected == nil || node.CurrentWeight > selected.CurrentWeight {
			selected = node
		}
	}

	if selected != nil {
		// 选中节点的当前权重 -= 总权重
		selected.CurrentWeight -= totalWeight
	}

	return selected
}

// ReportSuccess 报告成功
func (s *ChannelSelector) ReportSuccess(providerAccountID uint) {
	// TODO: record success to circuit breaker and auto-enable if disabled
}

// ReportFailure 报告失败
func (s *ChannelSelector) ReportFailure(providerAccountID uint) {
	// TODO: record failure to circuit breaker and auto-disable based on threshold
}

// ClearCache 清除所有缓存
// 注意：由于使用 gsr.Cacher 接口，无法遍历删除所有 key
// 这里通过设置 TTL 让缓存自动过期，如需立即清除可以重启服务
func (s *ChannelSelector) ClearCache() {
	s.logger.Info("channel selector cache will expire based on TTL")
}

// ClearCacheByChannelID 清除指定通道的缓存
func (s *ChannelSelector) ClearCacheByChannelID(channelID uint) {
	ctx := context.Background()

	// 清除常见的 messageType 组合
	messageTypes := []string{"sms", "email", "push", "voice", "wechat", ""}
	for _, msgType := range messageTypes {
		cacheKey := buildCacheKey(channelID, msgType)
		if err := s.cache.Del(ctx, cacheKey); err != nil {
			s.logger.Warn(fmt.Sprintf("failed to delete cache key %s: %v", cacheKey, err))
		}
	}

	s.logger.Info(fmt.Sprintf("channel selector cache cleared for channel id=%d", channelID))
}

// ClearCacheByKey 清除指定 key 的缓存
func (s *ChannelSelector) ClearCacheByKey(channelID uint, messageType string) {
	ctx := context.Background()
	cacheKey := buildCacheKey(channelID, messageType)

	if err := s.cache.Del(ctx, cacheKey); err != nil {
		s.logger.Warn(fmt.Sprintf("failed to delete cache key %s: %v", cacheKey, err))
	} else {
		s.logger.Info(fmt.Sprintf("channel selector cache cleared for key %s", cacheKey))
	}
}

// InvalidateCacheForBinding 当绑定配置变更时清除相关缓存
// 用于管理员在界面操作后立即生效
func (s *ChannelSelector) InvalidateCacheForBinding(channelID uint) {
	s.ClearCacheByChannelID(channelID)
}

// GetCacheKeyPrefix 获取缓存 key 前缀（用于外部清理）
func GetCacheKeyPrefix() string {
	return cacheKeyPrefix
}

// IsCacheKey 判断是否为通道选择器的缓存 key
func IsCacheKey(key string) bool {
	return strings.HasPrefix(key, cacheKeyPrefix)
}
