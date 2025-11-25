package selector

import (
	"context"
	"fmt"
	"sync"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
)

// ChannelNode 通道节点（带权重）
type ChannelNode struct {
	// 保留旧字段以兼容现有代码
	Relation        *model.ChannelProviderRelation
	Provider        *model.Provider
	CurrentWeight   int // 当前权重
	EffectiveWeight int // 有效权重

	// 新字段
	ChannelTemplateBinding *model.ChannelTemplateBinding // 通道模板绑定配置
	ProviderAccount        *model.ProviderAccount        // 服务商账号配置
}

// ChannelSelector 通道选择器
type ChannelSelector struct {
	logger                    gsr.Logger
	channelTemplateBindingDao *dao.ChannelTemplateBindingDAO
	providerAccountDAO        *dao.ProviderAccountDAO
	// 保留旧的DAO以兼容
	relationDao *dao.ChannelProviderRelationDAO
	providerDAO *dao.ProviderDAO
	cache       map[string][]*ChannelNode // 按业务通道ID缓存
	mu          sync.RWMutex
}

// NewChannelSelector 创建通道选择器
func NewChannelSelector() *ChannelSelector {
	h := helper.GetHelper()
	return &ChannelSelector{
		logger:                    h.GetLogger(),
		channelTemplateBindingDao: dao.NewChannelTemplateBindingDAO(),
		providerAccountDAO:        dao.NewProviderAccountDAO(),
		relationDao:               dao.NewChannelProviderRelationDAO(),
		providerDAO:               dao.NewProviderDAO(),
		cache:                     make(map[string][]*ChannelNode),
	}
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
	cacheKey := fmt.Sprintf("%d:%s", channelID, messageType)

	// 先从缓存读取
	s.mu.RLock()
	if nodes, ok := s.cache[cacheKey]; ok {
		s.mu.RUnlock()
		return s.filterAvailableNodes(nodes), nil
	}
	s.mu.RUnlock()

	// 缓存未命中，从数据库加载
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check
	if nodes, ok := s.cache[cacheKey]; ok {
		return s.filterAvailableNodes(nodes), nil
	}

	// 获取通道的所有模板绑定配置
	channelBindings, err := s.channelTemplateBindingDao.GetActiveByChannelID(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel template bindings: %w", err)
	}

	if len(channelBindings) == 0 {
		// 新系统没有配置，尝试使用旧系统（向后兼容）
		s.logger.Info(fmt.Sprintf("no channel template bindings found, falling back to legacy relations for channel_id=%d", channelID))
		return s.getChannelNodesLegacy(ctx, channelID, messageType)
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
			// 构造兼容的Relation（用于旧代码）
			Relation: &model.ChannelProviderRelation{
				Priority: ctb.Priority,
				Weight:   ctb.Weight,
				IsActive: ctb.IsActive,
			},
		}

		nodes = append(nodes, node)
	}

	// 缓存结果
	s.cache[cacheKey] = nodes

	return s.filterAvailableNodes(nodes), nil
}

// getChannelNodesLegacy 使用旧的ChannelProviderRelation获取通道节点（已废弃）
// 旧系统依赖已废弃的 provider_channels 表，建议迁移到新的 ChannelTemplateBinding 系统
func (s *ChannelSelector) getChannelNodesLegacy(ctx context.Context, channelID uint, messageType string) ([]*ChannelNode, error) {
	s.logger.Warn(fmt.Sprintf("legacy channel relations system is deprecated, please migrate to ChannelTemplateBinding for channel_id=%d", channelID))
	return nil, fmt.Errorf("no channel template bindings configured for channel_id=%d type=%s, legacy system is deprecated", channelID, messageType)
}

// filterAvailableNodes 过滤可用节点
func (s *ChannelSelector) filterAvailableNodes(nodes []*ChannelNode) []*ChannelNode {
	var available []*ChannelNode
	for _, node := range nodes {
		// 新系统：检查 ChannelTemplateBinding 状态
		if node.ChannelTemplateBinding != nil {
			if node.ChannelTemplateBinding.IsActive == 1 {
				available = append(available, node)
			}
			continue
		}

		// 旧系统：检查 Relation 优先级
		if node.Relation != nil && node.Relation.Priority > 0 {
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
		} else if node.Relation != nil {
			priority = node.Relation.Priority
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

// ClearCache 清除缓存
func (s *ChannelSelector) ClearCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = make(map[string][]*ChannelNode)
	s.logger.Info("channel selector cache cleared")
}

// ClearCacheByChannelID 清除指定通道的缓存
func (s *ChannelSelector) ClearCacheByChannelID(channelID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 删除所有包含该通道ID的缓存
	for key := range s.cache {
		if len(key) > 0 && key[0:len(fmt.Sprintf("%d", channelID))] == fmt.Sprintf("%d", channelID) {
			delete(s.cache, key)
		}
	}

	s.logger.Info(fmt.Sprintf("channel selector cache cleared for channel id=%d", channelID))
}
