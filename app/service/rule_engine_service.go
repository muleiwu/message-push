package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	internalHelper "cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/muleiwu/gsr"
)

// EvaluateRequest 规则评估请求
type EvaluateRequest struct {
	Scene        string          // 场景：send_failure / callback_failure
	ProviderCode string          // 供应商代码
	MessageType  string          // 消息类型
	ErrorCode    string          // 错误码
	ErrorMessage string          // 错误消息
	Task         *model.PushTask // 任务信息
}

// EvaluateResult 规则评估结果
type EvaluateResult struct {
	Action      string             // 动作：retry, switch_provider, fail, alert
	MatchedRule *model.FailureRule // 匹配到的规则
	HasMatch    bool               // 是否匹配到规则
}

// RuleEngineService 规则引擎服务
type RuleEngineService struct {
	logger  gsr.Logger
	ruleDAO *dao.FailureRuleDAO
	cache   *ruleCache
}

// ruleCache 规则缓存
type ruleCache struct {
	sync.RWMutex
	rules map[string][]*model.FailureRule // key: scene
}

var (
	ruleEngineInstance *RuleEngineService
	ruleEngineOnce     sync.Once
)

// GetRuleEngineService 获取规则引擎服务单例
func GetRuleEngineService() *RuleEngineService {
	ruleEngineOnce.Do(func() {
		ruleEngineInstance = &RuleEngineService{
			logger:  internalHelper.GetHelper().GetLogger(),
			ruleDAO: dao.NewFailureRuleDAO(),
			cache: &ruleCache{
				rules: make(map[string][]*model.FailureRule),
			},
		}
		// 初始化时加载规则到缓存
		ruleEngineInstance.RefreshCache()
	})
	return ruleEngineInstance
}

// NewRuleEngineService 创建规则引擎服务（用于测试）
func NewRuleEngineService() *RuleEngineService {
	return GetRuleEngineService()
}

// Evaluate 评估失败并返回推荐动作
func (s *RuleEngineService) Evaluate(ctx context.Context, req *EvaluateRequest) *EvaluateResult {
	// 获取缓存的规则
	rules := s.getCachedRules(req.Scene)
	if len(rules) == 0 {
		// 缓存为空时尝试刷新
		s.RefreshCache()
		rules = s.getCachedRules(req.Scene)
	}

	// 按优先级遍历规则，找到第一个匹配的
	for _, rule := range rules {
		if s.matchRule(rule, req) {
			s.logger.Info(fmt.Sprintf("rule matched rule_id=%d rule_name=%s action=%s scene=%s provider=%s error_code=%s",
				rule.ID, rule.Name, rule.Action, req.Scene, req.ProviderCode, req.ErrorCode))
			return &EvaluateResult{
				Action:      rule.Action,
				MatchedRule: rule,
				HasMatch:    true,
			}
		}
	}

	// 没有匹配的规则，返回默认动作
	s.logger.Info(fmt.Sprintf("no rule matched, using default action scene=%s provider=%s error_code=%s",
		req.Scene, req.ProviderCode, req.ErrorCode))
	return s.getDefaultResult(req.Scene)
}

// matchRule 检查请求是否匹配规则
func (s *RuleEngineService) matchRule(rule *model.FailureRule, req *EvaluateRequest) bool {
	// 1. 检查供应商代码（空表示匹配所有）
	if rule.ProviderCode != "" && rule.ProviderCode != req.ProviderCode {
		return false
	}

	// 2. 检查消息类型（空表示匹配所有）
	if rule.MessageType != "" && rule.MessageType != req.MessageType {
		return false
	}

	// 3. 检查错误码（支持逗号分隔多个）
	if rule.ErrorCode != "" {
		errorCodes := strings.Split(rule.ErrorCode, ",")
		matched := false
		for _, code := range errorCodes {
			code = strings.TrimSpace(code)
			if code == req.ErrorCode {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 4. 检查错误消息关键字（模糊匹配，不区分大小写）
	if rule.ErrorKeyword != "" {
		keywords := strings.Split(rule.ErrorKeyword, ",")
		matched := false
		errorMsgLower := strings.ToLower(req.ErrorMessage)
		for _, keyword := range keywords {
			keyword = strings.TrimSpace(strings.ToLower(keyword))
			if keyword != "" && strings.Contains(errorMsgLower, keyword) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 所有条件都匹配（或为空）
	return true
}

// getDefaultResult 获取默认评估结果
func (s *RuleEngineService) getDefaultResult(scene string) *EvaluateResult {
	switch scene {
	case model.RuleSceneSendFailure:
		// 发送失败默认重试
		return &EvaluateResult{
			Action:   model.RuleActionRetry,
			HasMatch: false,
		}
	case model.RuleSceneCallbackFailure:
		// 回调失败默认标记失败
		return &EvaluateResult{
			Action:   model.RuleActionFail,
			HasMatch: false,
		}
	default:
		return &EvaluateResult{
			Action:   model.RuleActionFail,
			HasMatch: false,
		}
	}
}

// getCachedRules 获取缓存的规则
func (s *RuleEngineService) getCachedRules(scene string) []*model.FailureRule {
	s.cache.RLock()
	defer s.cache.RUnlock()
	return s.cache.rules[scene]
}

// RefreshCache 刷新规则缓存
func (s *RuleEngineService) RefreshCache() {
	s.cache.Lock()
	defer s.cache.Unlock()

	// 加载发送失败规则
	sendRules, err := s.ruleDAO.GetActiveByScene(model.RuleSceneSendFailure)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to load send_failure rules: %v", err))
	} else {
		s.cache.rules[model.RuleSceneSendFailure] = sendRules
		s.logger.Info(fmt.Sprintf("loaded send_failure rules count=%d", len(sendRules)))
	}

	// 加载回调失败规则
	callbackRules, err := s.ruleDAO.GetActiveByScene(model.RuleSceneCallbackFailure)
	if err != nil {
		s.logger.Error(fmt.Sprintf("failed to load callback_failure rules: %v", err))
	} else {
		s.cache.rules[model.RuleSceneCallbackFailure] = callbackRules
		s.logger.Info(fmt.Sprintf("loaded callback_failure rules count=%d", len(callbackRules)))
	}
}

// GetRuleDAO 获取规则DAO（供管理接口使用）
func (s *RuleEngineService) GetRuleDAO() *dao.FailureRuleDAO {
	return s.ruleDAO
}
