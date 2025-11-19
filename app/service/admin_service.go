package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	apphelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// AdminService 管理后台服务
type AdminService struct{}

// NewAdminService 创建管理后台服务实例
func NewAdminService() *AdminService {
	return &AdminService{}
}

// generateRandomKey 生成随机密钥
func generateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateApplication 创建应用
func (s *AdminService) CreateApplication(req *dto.CreateApplicationRequest) (*dto.ApplicationResponse, error) {
	logger := helper.GetHelper().GetLogger()

	// 生成AppID和AppSecret
	appID, err := generateRandomKey(16)
	if err != nil {
		logger.Error("生成app_id失败")
		return nil, fmt.Errorf("failed to generate app_id: %w", err)
	}

	appSecret, err := generateRandomKey(32)
	if err != nil {
		logger.Error("生成app_secret失败")
		return nil, fmt.Errorf("failed to generate app_secret: %w", err)
	}

	// 加密AppSecret
	encryptedSecret, err := apphelper.EncryptAppSecret(appSecret)
	if err != nil {
		logger.Error("加密app_secret失败")
		return nil, fmt.Errorf("failed to encrypt app_secret: %w", err)
	}

	status := int8(req.Status)
	if status == 0 {
		status = 1 // 默认启用
	}

	app := &model.Application{
		AppID:      appID,
		AppSecret:  encryptedSecret,
		AppName:    req.Name,
		Status:     status,
		DailyQuota: 10000, // 默认每日配额
		RateLimit:  100,   // 默认QPS限制
	}

	if err := dao.CreateApp(app); err != nil {
		logger.Error("创建应用失败")
		return nil, err
	}

	logger.Info("应用创建成功")

	return &dto.ApplicationResponse{
		ID:          app.ID,
		Name:        app.AppName,
		Description: req.Description,
		AppKey:      appID,
		AppSecret:   appSecret, // 仅创建时返回明文
		Status:      int(app.Status),
		DailyLimit:  app.DailyQuota,
		QPSLimit:    app.RateLimit,
		CreatedAt:   app.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   app.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// GetApplicationList 获取应用列表
func (s *AdminService) GetApplicationList(req *dto.ApplicationListRequest) (*dto.ApplicationListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	var apps []*model.Application
	var total int64

	query := helper.GetHelper().GetDatabase().Model(&model.Application{})

	// 条件过滤
	if req.Name != "" {
		query = query.Where("app_name LIKE ?", "%"+req.Name+"%")
	}
	if req.Status > 0 {
		query = query.Where("status = ?", req.Status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count applications: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&apps).Error; err != nil {
		return nil, fmt.Errorf("failed to query applications: %w", err)
	}

	items := make([]*dto.ApplicationResponse, 0, len(apps))
	for _, app := range apps {
		items = append(items, &dto.ApplicationResponse{
			ID:          app.ID,
			Name:        app.AppName,
			Description: "",
			AppKey:      app.AppID,
			Status:      int(app.Status),
			DailyLimit:  app.DailyQuota,
			QPSLimit:    app.RateLimit,
			CreatedAt:   app.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   app.UpdatedAt.Format(time.RFC3339),
		})
	}

	return &dto.ApplicationListResponse{
		Total: int(total),
		Page:  page,
		Size:  pageSize,
		Items: items,
	}, nil
}

// GetApplicationByID 获取应用详情
func (s *AdminService) GetApplicationByID(id uint) (*dto.ApplicationResponse, error) {
	app, err := dao.GetAppByID(id)
	if err != nil {
		return nil, err
	}

	return &dto.ApplicationResponse{
		ID:          app.ID,
		Name:        app.AppName,
		Description: "",
		AppKey:      app.AppID,
		Status:      int(app.Status),
		DailyLimit:  app.DailyQuota,
		QPSLimit:    app.RateLimit,
		CreatedAt:   app.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   app.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// UpdateApplication 更新应用
func (s *AdminService) UpdateApplication(id uint, req *dto.UpdateApplicationRequest) error {
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["app_name"] = req.Name
	}
	if req.Status > 0 {
		updates["status"] = int8(req.Status)
	}

	if len(updates) == 0 {
		return nil
	}

	return dao.UpdateApp(id, updates)
}

// DeleteApplication 删除应用
func (s *AdminService) DeleteApplication(id uint) error {
	return dao.DeleteApp(id)
}

// RegenerateSecret 重新生成密钥
func (s *AdminService) RegenerateSecret(appID uint) (*dto.RegenerateSecretResponse, error) {
	logger := helper.GetHelper().GetLogger()

	app, err := dao.GetAppByID(appID)
	if err != nil {
		return nil, err
	}

	// 生成新的AppSecret
	appSecret, err := generateRandomKey(32)
	if err != nil {
		logger.Error("生成app_secret失败")
		return nil, fmt.Errorf("failed to generate app_secret: %w", err)
	}

	// 加密AppSecret
	encryptedSecret, err := apphelper.EncryptAppSecret(appSecret)
	if err != nil {
		logger.Error("加密app_secret失败")
		return nil, fmt.Errorf("failed to encrypt app_secret: %w", err)
	}

	// 更新数据库
	if err := dao.UpdateApp(appID, map[string]interface{}{
		"app_secret": encryptedSecret,
	}); err != nil {
		return nil, err
	}

	logger.Info("应用密钥重新生成成功")

	return &dto.RegenerateSecretResponse{
		AppKey:    app.AppID,
		AppSecret: appSecret,
	}, nil
}

// CreateProvider 创建服务商
func (s *AdminService) CreateProvider(req *dto.CreateProviderRequest) (*dto.ProviderResponse, error) {
	logger := helper.GetHelper().GetLogger()

	status := int8(req.Status)
	if status == 0 {
		status = 1 // 默认启用
	}

	// 生成provider_code
	providerCode, err := generateRandomKey(8)
	if err != nil {
		logger.Error("生成provider_code失败")
		return nil, fmt.Errorf("failed to generate provider_code: %w", err)
	}

	provider := &model.Provider{
		ProviderCode: providerCode,
		ProviderName: req.Name,
		ProviderType: req.Type,
		Status:       status,
		Remark:       req.Description,
	}

	// 设置配置
	if err := provider.SetConfig(req.Config); err != nil {
		logger.Error("设置服务商配置失败")
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	db := helper.GetHelper().GetDatabase()
	if err := db.Create(provider).Error; err != nil {
		logger.Error("创建服务商失败")
		return nil, err
	}

	logger.Info("服务商创建成功")

	config, _ := provider.GetConfig()
	return &dto.ProviderResponse{
		ID:          provider.ID,
		Name:        provider.ProviderName,
		Type:        provider.ProviderType,
		Description: req.Description,
		Config:      config,
		Status:      int(provider.Status),
		CreatedAt:   provider.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   provider.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// GetProviderList 获取服务商列表
func (s *AdminService) GetProviderList(req *dto.ProviderListRequest) (*dto.ProviderListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	var providers []*model.Provider
	var total int64

	query := helper.GetHelper().GetDatabase().Model(&model.Provider{})

	// 条件过滤
	if req.Type != "" {
		query = query.Where("provider_type = ?", req.Type)
	}
	if req.Status > 0 {
		query = query.Where("status = ?", req.Status)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count providers: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("id DESC").Find(&providers).Error; err != nil {
		return nil, fmt.Errorf("failed to query providers: %w", err)
	}

	items := make([]*dto.ProviderResponse, 0, len(providers))
	for _, provider := range providers {
		config, _ := provider.GetConfig()
		items = append(items, &dto.ProviderResponse{
			ID:          provider.ID,
			Name:        provider.ProviderName,
			Type:        provider.ProviderType,
			Description: provider.Remark,
			Config:      config,
			Status:      int(provider.Status),
			CreatedAt:   provider.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   provider.UpdatedAt.Format(time.RFC3339),
		})
	}

	return &dto.ProviderListResponse{
		Total: int(total),
		Page:  page,
		Size:  pageSize,
		Items: items,
	}, nil
}

// GetProviderByID 获取服务商详情
func (s *AdminService) GetProviderByID(id uint) (*dto.ProviderResponse, error) {
	var provider model.Provider
	db := helper.GetHelper().GetDatabase()
	if err := db.First(&provider, id).Error; err != nil {
		return nil, err
	}

	config, _ := provider.GetConfig()
	return &dto.ProviderResponse{
		ID:          provider.ID,
		Name:        provider.ProviderName,
		Type:        provider.ProviderType,
		Description: provider.Remark,
		Config:      config,
		Status:      int(provider.Status),
		CreatedAt:   provider.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   provider.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// UpdateProvider 更新服务商
func (s *AdminService) UpdateProvider(id uint, req *dto.UpdateProviderRequest) error {
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["provider_name"] = req.Name
	}
	if req.Description != "" {
		updates["remark"] = req.Description
	}
	if req.Status > 0 {
		updates["status"] = int8(req.Status)
	}
	if len(req.Config) > 0 {
		// 需要先查询现有Provider来处理配置
		var provider model.Provider
		db := helper.GetHelper().GetDatabase()
		if err := db.First(&provider, id).Error; err != nil {
			return err
		}
		if err := provider.SetConfig(req.Config); err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
		updates["config"] = provider.Config
	}

	if len(updates) == 0 {
		return nil
	}

	db := helper.GetHelper().GetDatabase()
	return db.Model(&model.Provider{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteProvider 删除服务商
func (s *AdminService) DeleteProvider(id uint) error {
	db := helper.GetHelper().GetDatabase()
	return db.Delete(&model.Provider{}, id).Error
}

// CreateChannel 创建通道
func (s *AdminService) CreateChannel(req *dto.CreateChannelRequest) (*dto.ChannelResponse, error) {
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
func (s *AdminService) GetChannelList(req *dto.ChannelListRequest) (*dto.ChannelListResponse, error) {
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
func (s *AdminService) GetChannelByID(id uint) (*dto.ChannelResponse, error) {
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
func (s *AdminService) UpdateChannel(id uint, req *dto.UpdateChannelRequest) error {
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
func (s *AdminService) DeleteChannel(id uint) error {
	db := helper.GetHelper().GetDatabase()
	return db.Delete(&model.Channel{}, id).Error
}

// BindProviderToChannel 绑定服务商到通道
func (s *AdminService) BindProviderToChannel(req *dto.BindProviderToChannelRequest) error {
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

// GetStatistics 获取推送统计
func (s *AdminService) GetStatistics(req *dto.StatisticsRequest) (*dto.StatisticsResponse, error) {
	db := helper.GetHelper().GetDatabase()

	// 基本查询
	query := db.Model(&model.PushLog{}).Where("DATE(created_at) >= ? AND DATE(created_at) <= ?", req.StartDate, req.EndDate)

	// 条件过滤
	if req.AppID > 0 {
		query = query.Where("app_id = ?", req.AppID)
	}
	if req.ChannelID > 0 {
		query = query.Where("provider_channel_id = ?", req.ChannelID)
	}

	// 汇总统计
	var summary struct {
		Total   int64
		Success int64
		Failed  int64
	}

	query.Count(&summary.Total)
	query.Where("status = ?", "success").Count(&summary.Success)
	summary.Failed = summary.Total - summary.Success

	// 每日统计
	var dailyStats []struct {
		Date    string
		Total   int64
		Success int64
	}

	db.Model(&model.PushLog{}).
		Select("DATE(created_at) as date, COUNT(*) as total, SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success").
		Where("DATE(created_at) >= ? AND DATE(created_at) <= ?", req.StartDate, req.EndDate).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&dailyStats)

	// 构建响应
	response := &dto.StatisticsResponse{}
	response.Summary.TotalCount = summary.Total
	response.Summary.SuccessCount = summary.Success
	response.Summary.FailureCount = summary.Failed
	if summary.Total > 0 {
		response.Summary.SuccessRate = fmt.Sprintf("%.2f%%", float64(summary.Success)/float64(summary.Total)*100)
	} else {
		response.Summary.SuccessRate = "0.00%"
	}

	response.Daily = make([]*dto.DailyStatistics, 0, len(dailyStats))
	for _, stat := range dailyStats {
		failed := stat.Total - stat.Success
		successRate := "0.00%"
		if stat.Total > 0 {
			successRate = fmt.Sprintf("%.2f%%", float64(stat.Success)/float64(stat.Total)*100)
		}
		response.Daily = append(response.Daily, &dto.DailyStatistics{
			Date:         stat.Date,
			TotalCount:   stat.Total,
			SuccessCount: stat.Success,
			FailureCount: failed,
			SuccessRate:  successRate,
		})
	}

	return response, nil
}
