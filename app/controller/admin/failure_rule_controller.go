package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"cnb.cool/mliev/push/message-push/app/controller"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/service"
	"cnb.cool/mliev/push/message-push/internal/interfaces"
)

// FailureRuleController 失败规则管理控制器
type FailureRuleController struct{}

// CreateFailureRule 创建失败规则
func (c FailureRuleController) CreateFailureRule(ctx *gin.Context, helper interfaces.HelperInterface) {
	var req dto.CreateFailureRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	// 验证场景
	if !model.IsValidRuleScene(req.Scene) {
		controller.ErrorResponse(ctx, 400, "invalid scene")
		return
	}

	// 验证动作
	if !model.IsValidRuleAction(req.Action) {
		controller.ErrorResponse(ctx, 400, "invalid action")
		return
	}

	ruleEngine := service.GetRuleEngineService()
	ruleDAO := ruleEngine.GetRuleDAO()

	rule := &model.FailureRule{
		Name:         req.Name,
		Scene:        req.Scene,
		ProviderCode: req.ProviderCode,
		MessageType:  req.MessageType,
		ErrorCode:    req.ErrorCode,
		ErrorKeyword: req.ErrorKeyword,
		Action:       req.Action,
		ActionConfig: req.ActionConfig,
		Priority:     req.Priority,
		Status:       int8(req.Status),
		Remark:       req.Remark,
	}

	// 默认启用
	if req.Status == 0 {
		rule.Status = 1
	}

	if err := ruleDAO.Create(rule); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to create rule: "+err.Error())
		return
	}

	// 刷新缓存
	ruleEngine.RefreshCache()

	controller.SuccessResponse(ctx, toFailureRuleResponse(rule))
}

// GetFailureRuleList 获取失败规则列表
func (c FailureRuleController) GetFailureRuleList(ctx *gin.Context, helper interfaces.HelperInterface) {
	var req dto.FailureRuleListRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	ruleEngine := service.GetRuleEngineService()
	ruleDAO := ruleEngine.GetRuleDAO()

	rules, total, err := ruleDAO.List(req.Page, req.PageSize, req.Scene)
	if err != nil {
		controller.ErrorResponse(ctx, 500, "failed to get rule list: "+err.Error())
		return
	}

	items := make([]*dto.FailureRuleResponse, 0, len(rules))
	for _, rule := range rules {
		items = append(items, toFailureRuleResponse(rule))
	}

	controller.SuccessResponse(ctx, &dto.FailureRuleListResponse{
		Total: int(total),
		Page:  req.Page,
		Size:  req.PageSize,
		Items: items,
	})
}

// GetFailureRule 获取失败规则详情
func (c FailureRuleController) GetFailureRule(ctx *gin.Context, helper interfaces.HelperInterface) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	ruleEngine := service.GetRuleEngineService()
	ruleDAO := ruleEngine.GetRuleDAO()

	rule, err := ruleDAO.GetByID(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 404, "rule not found")
		return
	}

	controller.SuccessResponse(ctx, toFailureRuleResponse(rule))
}

// UpdateFailureRule 更新失败规则
func (c FailureRuleController) UpdateFailureRule(ctx *gin.Context, helper interfaces.HelperInterface) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	var req dto.UpdateFailureRuleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		controller.ErrorResponse(ctx, 400, "invalid request: "+err.Error())
		return
	}

	ruleEngine := service.GetRuleEngineService()
	ruleDAO := ruleEngine.GetRuleDAO()

	rule, err := ruleDAO.GetByID(uint(id))
	if err != nil {
		controller.ErrorResponse(ctx, 404, "rule not found")
		return
	}

	// 更新字段
	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.Scene != "" {
		if !model.IsValidRuleScene(req.Scene) {
			controller.ErrorResponse(ctx, 400, "invalid scene")
			return
		}
		rule.Scene = req.Scene
	}
	if req.Action != "" {
		if !model.IsValidRuleAction(req.Action) {
			controller.ErrorResponse(ctx, 400, "invalid action")
			return
		}
		rule.Action = req.Action
	}

	// 允许清空的字段
	rule.ProviderCode = req.ProviderCode
	rule.MessageType = req.MessageType
	rule.ErrorCode = req.ErrorCode
	rule.ErrorKeyword = req.ErrorKeyword
	rule.ActionConfig = req.ActionConfig
	rule.Remark = req.Remark

	if req.Priority > 0 {
		rule.Priority = req.Priority
	}
	if req.Status != nil {
		rule.Status = int8(*req.Status)
	}

	if err := ruleDAO.Update(rule); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to update rule: "+err.Error())
		return
	}

	// 刷新缓存
	ruleEngine.RefreshCache()

	controller.SuccessResponse(ctx, gin.H{"message": "updated successfully"})
}

// DeleteFailureRule 删除失败规则
func (c FailureRuleController) DeleteFailureRule(ctx *gin.Context, helper interfaces.HelperInterface) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		controller.ErrorResponse(ctx, 400, "invalid id")
		return
	}

	ruleEngine := service.GetRuleEngineService()
	ruleDAO := ruleEngine.GetRuleDAO()

	if err := ruleDAO.Delete(uint(id)); err != nil {
		controller.ErrorResponse(ctx, 500, "failed to delete rule: "+err.Error())
		return
	}

	// 刷新缓存
	ruleEngine.RefreshCache()

	controller.SuccessResponse(ctx, gin.H{"message": "deleted successfully"})
}

// GetFailureRuleOptions 获取失败规则选项
func (c FailureRuleController) GetFailureRuleOptions(ctx *gin.Context, helper interfaces.HelperInterface) {
	controller.SuccessResponse(ctx, &dto.FailureRuleOptionsResponse{
		Scenes: []dto.OptionItem{
			{Value: model.RuleSceneSendFailure, Label: "发送失败"},
			{Value: model.RuleSceneCallbackFailure, Label: "回调失败"},
		},
		Actions: []dto.OptionItem{
			{Value: model.RuleActionRetry, Label: "重试"},
			{Value: model.RuleActionSwitchProvider, Label: "切换供应商"},
			{Value: model.RuleActionFail, Label: "标记失败"},
			{Value: model.RuleActionAlert, Label: "告警通知"},
		},
	})
}

// RefreshRuleCache 刷新规则缓存
func (c FailureRuleController) RefreshRuleCache(ctx *gin.Context, helper interfaces.HelperInterface) {
	ruleEngine := service.GetRuleEngineService()
	ruleEngine.RefreshCache()
	controller.SuccessResponse(ctx, gin.H{"message": "cache refreshed"})
}

// toFailureRuleResponse 转换为响应DTO
func toFailureRuleResponse(rule *model.FailureRule) *dto.FailureRuleResponse {
	return &dto.FailureRuleResponse{
		ID:           rule.ID,
		Name:         rule.Name,
		Scene:        rule.Scene,
		SceneLabel:   getSceneLabel(rule.Scene),
		ProviderCode: rule.ProviderCode,
		MessageType:  rule.MessageType,
		ErrorCode:    rule.ErrorCode,
		ErrorKeyword: rule.ErrorKeyword,
		Action:       rule.Action,
		ActionLabel:  getActionLabel(rule.Action),
		ActionConfig: rule.ActionConfig,
		Priority:     rule.Priority,
		Status:       rule.Status,
		Remark:       rule.Remark,
		CreatedAt:    rule.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    rule.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// getSceneLabel 获取场景标签
func getSceneLabel(scene string) string {
	switch scene {
	case model.RuleSceneSendFailure:
		return "发送失败"
	case model.RuleSceneCallbackFailure:
		return "回调失败"
	default:
		return scene
	}
}

// getActionLabel 获取动作标签
func getActionLabel(action string) string {
	switch action {
	case model.RuleActionRetry:
		return "重试"
	case model.RuleActionSwitchProvider:
		return "切换供应商"
	case model.RuleActionFail:
		return "标记失败"
	case model.RuleActionAlert:
		return "告警通知"
	default:
		return action
	}
}
