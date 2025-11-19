package service

import (
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	apphelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// AdminApplicationService 应用管理服务
type AdminApplicationService struct{}

// NewAdminApplicationService 创建应用管理服务实例
func NewAdminApplicationService() *AdminApplicationService {
	return &AdminApplicationService{}
}

// CreateApplication 创建应用
func (s *AdminApplicationService) CreateApplication(req *dto.CreateApplicationRequest) (*dto.ApplicationResponse, error) {
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
func (s *AdminApplicationService) GetApplicationList(req *dto.ApplicationListRequest) (*dto.ApplicationListResponse, error) {
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
func (s *AdminApplicationService) GetApplicationByID(id uint) (*dto.ApplicationResponse, error) {
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
func (s *AdminApplicationService) UpdateApplication(id uint, req *dto.UpdateApplicationRequest) error {
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
func (s *AdminApplicationService) DeleteApplication(id uint) error {
	return dao.DeleteApp(id)
}

// RegenerateSecret 重新生成密钥
func (s *AdminApplicationService) RegenerateSecret(appID uint) (*dto.RegenerateSecretResponse, error) {
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
