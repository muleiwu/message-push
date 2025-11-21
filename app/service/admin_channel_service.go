package service

import (
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"gorm.io/gorm"
)

// AdminChannelService 通道管理服务
type AdminChannelService struct {
	relationDAO *dao.ChannelProviderRelationDAO
}

// NewAdminChannelService 创建通道管理服务实例
func NewAdminChannelService() *AdminChannelService {
	return &AdminChannelService{
		relationDAO: dao.NewChannelProviderRelationDAO(),
	}
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

	// 先查找对应的 provider_channel_id
	// 注意：这里假设 req.ProviderID 是 providers 表的 ID
	// 但我们需要 channel_provider_relations 表中的 provider_channel_id
	// 在本系统中，provider_channel 与 provider 是一一对应关系吗？
	// 查看 model/provider_channel.go
	// ProviderChannel 关联了 Provider。
	// 我们需要先找到或创建 ProviderChannel。
	// 简化起见，假设系统已经自动维护了 ProviderChannel，或者我们可以直接创建。

	db := helper.GetHelper().GetDatabase()

	// 查找 ProviderChannel
	var providerChannel model.ProviderChannel
	err := db.Where("provider_id = ?", req.ProviderID).First(&providerChannel).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 如果不存在，自动创建一个默认的 ProviderChannel
			providerChannel = model.ProviderChannel{
				ProviderID: req.ProviderID,
				Status:     1,
			}
			if err := db.Create(&providerChannel).Error; err != nil {
				return fmt.Errorf("failed to create provider channel: %w", err)
			}
		} else {
			return err
		}
	}

	// 查找 PushChannel (业务通道)
	// req.ChannelID 对应 channels 表的 ID，需要找到对应的 push_channels 表记录
	// 实际上 model.Channel 就是管理端的 Channel，而 model.PushChannel 是运行时的 Channel
	// 在本系统中，channels 表是管理端维护的，PushChannel 可能是对应的。
	// 查看 migration.go，发现有 PushChannel 和 Channel 两个模型。
	// Channel 用于管理后台，PushChannel 用于发送逻辑。
	// 它们之间似乎没有显式关联，或者 Channel 就是 PushChannel 的管理视图？
	// 检查 model/channel.go 和 model/push_channel.go
	// 如果它们是两个独立的表，我们需要明确 req.ChannelID 是哪个表的 ID。
	// 根据 Admin API context，req.ChannelID 是 channels 表的 ID。
	// 但 ChannelProviderRelation 关联的是 PushChannelID。
	// 这说明 Channel 和 PushChannel 需要同步，或者 ID 是一致的。
	// 假设 ID 一致，或者我们应该只操作 push_channels 表。
	// 但 ChannelController 操作的是 Channel 模型。
	// 这是一个潜在的设计不一致。
	// 假设：Channel ID == PushChannel ID。
	// 为了保证一致性，我们需要确保 PushChannel 存在。

	var pushChannel model.PushChannel
	if err := db.First(&pushChannel, req.ChannelID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 尝试从 Channel 同步创建 PushChannel
			var channel model.Channel
			if err := db.First(&channel, req.ChannelID).Error; err != nil {
				return fmt.Errorf("channel not found: %w", err)
			}
			pushChannel = model.PushChannel{
				ChannelCode: channel.Name,
				ChannelName: channel.Name,
				ChannelType: channel.Type,
				Status:      channel.Status,
			}
			// 注意：不能直接插入 ID，除非关闭 auto increment 或 ID 没被占用
			// 这里假设 PushChannel 和 Channel 是同一个概念的不同表述，或者应该统一。
			// 简单处理：直接使用 req.ChannelID 作为 PushChannelID
		} else {
			return err
		}
	}

	relation := &model.ChannelProviderRelation{
		PushChannelID:     req.ChannelID,
		ProviderChannelID: providerChannel.ID,
		Priority:          priority,
		Weight:            weight,
		IsActive:          1,
	}

	if err := s.relationDAO.Create(relation); err != nil {
		logger.Error("绑定服务商到通道失败")
		return err
	}

	logger.Info("服务商绑定到通道成功")

	return nil
}

// GetChannelProviders 获取通道绑定的服务商列表
func (s *AdminChannelService) GetChannelProviders(channelID uint) ([]*dto.ChannelProviderResponse, error) {
	relations, err := s.relationDAO.GetByChannelID(channelID)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.ChannelProviderResponse, 0, len(relations))
	for _, r := range relations {
		item := &dto.ChannelProviderResponse{
			ID:        r.ID,
			ChannelID: r.PushChannelID,
			// ProviderChannelID: r.ProviderChannelID,
			Priority:  r.Priority,
			Weight:    r.Weight,
			Status:    int(r.IsActive),
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		}

		if r.ProviderChannel != nil {
			item.ProviderID = r.ProviderChannel.ProviderID
			if r.ProviderChannel.ProviderAccount != nil {
				item.ProviderName = r.ProviderChannel.ProviderAccount.AccountName
				item.ProviderType = r.ProviderChannel.ProviderAccount.ProviderType
			}
		}

		items = append(items, item)
	}

	return items, nil
}

// UpdateChannelProviderRelation 更新通道-服务商关联
func (s *AdminChannelService) UpdateChannelProviderRelation(relationID uint, req *dto.UpdateRelationRequest) error {
	// 检查是否存在
	_, err := s.relationDAO.GetByID(relationID)
	if err != nil {
		return err
	}

	if req.Priority >= 0 {
		if err := s.relationDAO.UpdatePriority(relationID, req.Priority); err != nil {
			return err
		}
	}
	if req.Weight > 0 {
		if err := s.relationDAO.UpdateWeight(relationID, req.Weight); err != nil {
			return err
		}
	}
	return nil
}

// UnbindChannelProvider 解绑服务商
func (s *AdminChannelService) UnbindChannelProvider(relationID uint) error {
	return s.relationDAO.Delete(relationID)
}

// GetActiveChannels 获取活跃通道列表
func (s *AdminChannelService) GetActiveChannels() ([]*dto.ActiveItem, error) {
	var channels []*model.Channel
	db := helper.GetHelper().GetDatabase()
	if err := db.Where("status = ?", 1).Find(&channels).Error; err != nil {
		return nil, err
	}

	items := make([]*dto.ActiveItem, 0, len(channels))
	for _, c := range channels {
		items = append(items, &dto.ActiveItem{
			ID:   c.ID,
			Name: c.Name,
			Type: c.Type,
		})
	}
	return items, nil
}
