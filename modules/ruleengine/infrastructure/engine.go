// Package infrastructure 提供 ruleengine 领域端口的实现：
// 从数据库加载失败规则并缓存于内存，按优先级匹配。
package infrastructure

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/ruleengine/domain"
	"github.com/muleiwu/gsr"
)

// 确保 RuleEngineService 实现 domain.RuleEngine 端口
var _ domain.RuleEngine = (*RuleEngineService)(nil)

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

// New 创建规则引擎服务并预加载规则缓存。
func New() *RuleEngineService {
	s := &RuleEngineService{
		logger:  helper.GetLogger(),
		ruleDAO: dao.NewFailureRuleDAO(),
		cache: &ruleCache{
			rules: make(map[string][]*model.FailureRule),
		},
	}
	s.RefreshCache()
	return s
}

// Evaluate 评估失败并返回推荐动作
func (s *RuleEngineService) Evaluate(ctx context.Context, req *domain.EvaluateRequest) *domain.EvaluateResult {
	rules := s.getCachedRules(req.Scene)
	if len(rules) == 0 {
		s.RefreshCache()
		rules = s.getCachedRules(req.Scene)
	}

	for _, rule := range rules {
		if s.matchRule(rule, req) {
			s.logger.Info(fmt.Sprintf("rule matched rule_id=%d rule_name=%s action=%s scene=%s provider=%s error_code=%s",
				rule.ID, rule.Name, rule.Action, req.Scene, req.ProviderCode, req.ErrorCode))
			return &domain.EvaluateResult{
				Action:      rule.Action,
				MatchedRule: rule,
				HasMatch:    true,
			}
		}
	}

	s.logger.Info(fmt.Sprintf("no rule matched, using default action scene=%s provider=%s error_code=%s",
		req.Scene, req.ProviderCode, req.ErrorCode))
	return s.getDefaultResult(req.Scene)
}

// matchRule 检查请求是否匹配规则
func (s *RuleEngineService) matchRule(rule *model.FailureRule, req *domain.EvaluateRequest) bool {
	if rule.ProviderCode != "" && rule.ProviderCode != req.ProviderCode {
		return false
	}

	if rule.MessageType != "" && rule.MessageType != req.MessageType {
		return false
	}

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

	return true
}

// getDefaultResult 获取默认评估结果
func (s *RuleEngineService) getDefaultResult(scene string) *domain.EvaluateResult {
	switch scene {
	case model.RuleSceneSendFailure:
		return &domain.EvaluateResult{
			Action:   model.RuleActionRetry,
			HasMatch: false,
		}
	case model.RuleSceneCallbackFailure:
		return &domain.EvaluateResult{
			Action:   model.RuleActionFail,
			HasMatch: false,
		}
	default:
		return &domain.EvaluateResult{
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

	sendRules, err := s.ruleDAO.GetActiveByScene(model.RuleSceneSendFailure)
	if err != nil {
		s.logRefreshError("send_failure", err)
	} else {
		s.cache.rules[model.RuleSceneSendFailure] = sendRules
		s.logger.Info(fmt.Sprintf("loaded send_failure rules count=%d", len(sendRules)))
	}

	callbackRules, err := s.ruleDAO.GetActiveByScene(model.RuleSceneCallbackFailure)
	if err != nil {
		s.logRefreshError("callback_failure", err)
	} else {
		s.cache.rules[model.RuleSceneCallbackFailure] = callbackRules
		s.logger.Info(fmt.Sprintf("loaded callback_failure rules count=%d", len(callbackRules)))
	}
}

// logRefreshError 记录规则加载失败。首次启动时 RefreshCache 在数据库迁移之前
// 于装配阶段执行，failure_rules 表尚未创建，此时降级为 Warn，避免误导性的错误日志。
func (s *RuleEngineService) logRefreshError(scene string, err error) {
	if isUndefinedTableErr(err) {
		s.logger.Warn(fmt.Sprintf("skip loading %s rules: failure_rules table not ready yet (will load after migration): %v", scene, err))
		return
	}
	s.logger.Error(fmt.Sprintf("failed to load %s rules: %v", scene, err))
}

// isUndefinedTableErr 判断是否为「表不存在」错误（PostgreSQL 42P01 / MySQL 1146）。
func isUndefinedTableErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "42p01") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "doesn't exist")
}
