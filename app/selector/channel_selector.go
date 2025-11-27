package selector

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
)

// 缓存 key 前缀
const (
	cacheKeyPrefix  = "channel_selector:"
	weightKeyPrefix = "channel_weight:"
)

// 权重状态 TTL（24小时作为兜底，管理操作会主动清除）
const weightTTL = 24 * time.Hour

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

// Select 选择通道（平滑加权轮询，权重状态持久化到 Redis）
func (s *ChannelSelector) Select(ctx context.Context, channelID uint, messageType string) (*ChannelNode, error) {
	nodes, err := s.getChannelNodes(ctx, channelID, messageType)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no available channel for channel_id=%d type=%s", channelID, messageType)
	}

	// 使用平滑加权轮询选择（权重状态持久化到 Redis）
	selected := s.smoothWeightedRoundRobin(ctx, channelID, nodes)
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

// smoothWeightedRoundRobin 平滑加权轮询算法（权重状态持久化到 Redis）
func (s *ChannelSelector) smoothWeightedRoundRobin(ctx context.Context, channelID uint, nodes []*ChannelNode) *ChannelNode {
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

	// 从 Redis 加载各节点的当前权重状态
	s.loadWeightStates(ctx, channelID, candidates)

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

	// 保存更新后的权重状态到 Redis
	s.saveWeightStates(ctx, channelID, candidates)

	return selected
}

// buildWeightKey 构建权重状态缓存 key
func buildWeightKey(channelID uint, bindingID uint) string {
	return fmt.Sprintf("%s%d:%d", weightKeyPrefix, channelID, bindingID)
}

// loadWeightStates 从 Redis 批量加载权重状态
func (s *ChannelSelector) loadWeightStates(ctx context.Context, channelID uint, nodes []*ChannelNode) {
	for _, node := range nodes {
		if node.ChannelTemplateBinding == nil {
			continue
		}

		key := buildWeightKey(channelID, node.ChannelTemplateBinding.ID)
		var weightStr string
		err := s.cache.Get(ctx, key, &weightStr)
		if err == nil && weightStr != "" {
			if weight, parseErr := strconv.Atoi(weightStr); parseErr == nil {
				node.CurrentWeight = weight
			}
		}
		// 如果获取失败或解析失败，使用默认值 0
	}
}

// saveWeightStates 批量保存权重状态到 Redis
func (s *ChannelSelector) saveWeightStates(ctx context.Context, channelID uint, nodes []*ChannelNode) {
	for _, node := range nodes {
		if node.ChannelTemplateBinding == nil {
			continue
		}

		key := buildWeightKey(channelID, node.ChannelTemplateBinding.ID)
		weightStr := strconv.Itoa(node.CurrentWeight)
		if err := s.cache.Set(ctx, key, weightStr, weightTTL); err != nil {
			s.logger.Warn(fmt.Sprintf("failed to save weight state key=%s: %v", key, err))
		}
	}
}

// ResetWeightsByChannelID 重置指定通道的所有权重状态
// 在管理员操作（新增、修改、删除绑定）后调用
func (s *ChannelSelector) ResetWeightsByChannelID(channelID uint) {
	ctx := context.Background()

	// 从数据库获取该通道的所有绑定配置
	bindings, err := s.channelTemplateBindingDao.GetByChannelID(channelID)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("failed to get bindings for weight reset channel_id=%d: %v", channelID, err))
		return
	}

	// 删除每个绑定的权重状态
	for _, binding := range bindings {
		key := buildWeightKey(channelID, binding.ID)
		if err := s.cache.Del(ctx, key); err != nil {
			s.logger.Warn(fmt.Sprintf("failed to delete weight state key=%s: %v", key, err))
		}
	}

	s.logger.Info(fmt.Sprintf("weight states reset for channel_id=%d, cleared %d bindings", channelID, len(bindings)))
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
