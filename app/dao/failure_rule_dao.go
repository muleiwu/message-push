package dao

import (
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// FailureRuleDAO 失败规则数据访问对象
type FailureRuleDAO struct {
	db *gorm.DB
}

// NewFailureRuleDAO 创建FailureRuleDAO
func NewFailureRuleDAO() *FailureRuleDAO {
	return &FailureRuleDAO{
		db: helper.GetHelper().GetDatabase(),
	}
}

// Create 创建规则
func (d *FailureRuleDAO) Create(rule *model.FailureRule) error {
	return d.db.Create(rule).Error
}

// GetByID 根据ID获取规则
func (d *FailureRuleDAO) GetByID(id uint) (*model.FailureRule, error) {
	var rule model.FailureRule
	err := d.db.Where("id = ?", id).First(&rule).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// Update 更新规则
func (d *FailureRuleDAO) Update(rule *model.FailureRule) error {
	return d.db.Save(rule).Error
}

// Delete 删除规则（软删除）
func (d *FailureRuleDAO) Delete(id uint) error {
	return d.db.Delete(&model.FailureRule{}, id).Error
}

// List 获取规则列表（分页）
func (d *FailureRuleDAO) List(page, pageSize int, scene string) ([]*model.FailureRule, int64, error) {
	var rules []*model.FailureRule
	var total int64

	offset := (page - 1) * pageSize
	query := d.db.Model(&model.FailureRule{})

	if scene != "" {
		query = query.Where("scene = ?", scene)
	}

	// 计算总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 查询列表
	err := query.Offset(offset).Limit(pageSize).Order("priority DESC, id DESC").Find(&rules).Error
	if err != nil {
		return nil, 0, err
	}

	return rules, total, nil
}

// GetActiveByScene 获取指定场景的所有启用规则（按优先级降序）
func (d *FailureRuleDAO) GetActiveByScene(scene string) ([]*model.FailureRule, error) {
	var rules []*model.FailureRule
	err := d.db.Where("scene = ? AND status = 1", scene).
		Order("priority DESC, id DESC").
		Find(&rules).Error
	if err != nil {
		return nil, err
	}
	return rules, nil
}

// GetActiveBySceneAndProvider 获取指定场景和供应商的启用规则
func (d *FailureRuleDAO) GetActiveBySceneAndProvider(scene, providerCode string) ([]*model.FailureRule, error) {
	var rules []*model.FailureRule
	err := d.db.Where("scene = ? AND status = 1 AND (provider_code = ? OR provider_code = '')", scene, providerCode).
		Order("priority DESC, id DESC").
		Find(&rules).Error
	if err != nil {
		return nil, err
	}
	return rules, nil
}

// GetAll 获取所有规则
func (d *FailureRuleDAO) GetAll() ([]*model.FailureRule, error) {
	var rules []*model.FailureRule
	err := d.db.Order("priority DESC, id DESC").Find(&rules).Error
	if err != nil {
		return nil, err
	}
	return rules, nil
}

// GetByStatus 根据状态获取规则
func (d *FailureRuleDAO) GetByStatus(status int8) ([]*model.FailureRule, error) {
	var rules []*model.FailureRule
	err := d.db.Where("status = ?", status).Order("priority DESC").Find(&rules).Error
	if err != nil {
		return nil, err
	}
	return rules, nil
}

// BatchCreate 批量创建规则
func (d *FailureRuleDAO) BatchCreate(rules []*model.FailureRule) error {
	return d.db.Create(&rules).Error
}

// CountByScene 统计指定场景的规则数量
func (d *FailureRuleDAO) CountByScene(scene string) (int64, error) {
	var count int64
	err := d.db.Model(&model.FailureRule{}).Where("scene = ?", scene).Count(&count).Error
	return count, err
}
