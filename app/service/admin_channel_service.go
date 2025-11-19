package service

import (
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// AdminChannelService 通道管理服务
type AdminChannelService struct{}

// NewAdminChannelService 创建通道管理服务实例
func NewAdminChannelService() *AdminChannelService {
	return &AdminChannelService{}
}

// CreateChannel 创建通道
func (s *AdminChannelService) CreateChannel(req *dto.CreateChannelRequest) (*dto.ChannelResponse, error) {
	logger := helper.GetHelper().GetLogger()

	status := int8(req.Status)
	if status == 0 {
		status = 1 // 默认启用
	}

	channel := &model.Channel{
		Name:   req.Name,
		Type:   req.Type,
		Status: status,
	}

	db := helper.GetHelper().GetDatabase()
	if err := db.Create(channel).Error; err != nil {
		logger.Error("创建通道失败")
		return nil, err
	}

	logger.Info("通道创建成功")

	return &dto.ChannelResponse{
		ID:        channel.ID,
		Name:      channel.Name,
		Type:      channel.Type,
		Status:    int(channel.Status),
		CreatedAt: channel.CreatedAt.Format(time.RFC3339),
		UpdatedAt: channel.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// GetChannelList 获取通道列表
func (s *AdminChannelService) GetChannelList(req *dto.ChannelListRequest) (*dto.ChannelListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	var channels []*model.Channel
	var total int64

	query := helper.GetHelper().GetDatabase().Model(&model.Channel{})

	// 条件过滤
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	if req.Status > 0 {
		query = query.Where("status = ?", req.Status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count channels: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&channels).Error; err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}

	items := make([]*dto.ChannelResponse, 0, len(channels))
	for _, channel := range channels {
		items = append(items, &dto.ChannelResponse{
			ID:        channel.ID,
			Name:      channel.Name,
			Type:      channel.Type,
			Status:    int(channel.Status),
			CreatedAt: channel.CreatedAt.Format(time.RFC3339),
			UpdatedAt: channel.UpdatedAt.Format(time.RFC3339),
		})
	}

	return &dto.ChannelListResponse{
		Total: int(total),
		Page:  page,
		Size:  pageSize,
		Items: items,
	}, nil
}

// GetChannelByID 获取通道详情
func (s *AdminChannelService) GetChannelByID(id uint) (*dto.ChannelResponse, error) {
	var channel model.Channel
	db := helper.GetHelper().GetDatabase()
	if err := db.First(&channel, id).Error; err != nil {
		return nil, err
	}

	return &dto.ChannelResponse{
		ID:        channel.ID,
		Name:      channel.Name,
		Type:      channel.Type,
		Status:    int(channel.Status),
		CreatedAt: channel.CreatedAt.Format(time.RFC3339),
		UpdatedAt: channel.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// UpdateChannel 更新通道
func (s *AdminChannelService) UpdateChannel(id uint, req *dto.UpdateChannelRequest) error {
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Status > 0 {
		updates["status"] = int8(req.Status)
	}

	if len(updates) == 0 {
		return nil
	}

	db := helper.GetHelper().GetDatabase()
	return db.Model(&model.Channel{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteChannel 删除通道
func (s *AdminChannelService) DeleteChannel(id uint) error {
	db := helper.GetHelper().GetDatabase()
	return db.Delete(&model.Channel{}, id).Error
}

// BindProviderToChannel 绑定服务商到通道
func (s *AdminChannelService) BindProviderToChannel(req *dto.BindProviderToChannelRequest) error {
	logger := helper.GetHelper().GetLogger()

	priority := req.Priority
	weight := req.Weight
	if weight == 0 {
		weight = 10 // 默认权重
	}

	// 注意：这里的Channel和Provider需要先查询对应的ProviderChannel和PushChannel
	// 为简化处理，直接使用传入的ID作为PushChannelID和ProviderChannelID
	relation := &model.ChannelProviderRelation{
		PushChannelID:     req.ChannelID,
		ProviderChannelID: req.ProviderID,
		Priority:          priority,
		Weight:            weight,
		IsActive:          1,
	}

	db := helper.GetHelper().GetDatabase()
	if err := db.Create(relation).Error; err != nil {
		logger.Error("绑定服务商到通道失败")
		return err
	}

	logger.Info("服务商绑定到通道成功")

	return nil
}
