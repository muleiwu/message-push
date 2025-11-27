package service

import (
	"context"
	"fmt"
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	appHelper "cnb.cool/mliev/push/message-push/app/helper"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/registry"
	"cnb.cool/mliev/push/message-push/app/sender"
	"cnb.cool/mliev/push/message-push/internal/helper"
	"github.com/gin-gonic/gin"
)

// AdminProviderAccountService 服务商账号配置管理服务
type AdminProviderAccountService struct{}

// NewAdminProviderAccountService 创建服务商账号管理服务实例
func NewAdminProviderAccountService() *AdminProviderAccountService {
	return &AdminProviderAccountService{}
}

// generateCallbackURL 生成回调地址
// 仅当服务商支持回调时返回URL，否则返回空字符串
func (s *AdminProviderAccountService) generateCallbackURL(c *gin.Context, accountID uint, providerCode string) string {
	// 从注册中心检查是否支持回调
	meta, err := registry.GetByCode(providerCode)
	if err != nil || !meta.SupportsCallback {
		return ""
	}

	// 生成回调 URL
	return appHelper.GetBaseURL(c, fmt.Sprintf("/api/callback/%d", accountID))
}

// GetAvailableProviders 获取可用的服务商列表（从注册中心）
func (s *AdminProviderAccountService) GetAvailableProviders(providerType string) ([]*dto.AvailableProviderResponse, error) {
	var providers []*registry.ProviderMeta

	if providerType != "" {
		// 按类型过滤
		providers = registry.GetByType(providerType)
	} else {
		// 获取所有
		providers = registry.GetAll()
	}

	result := make([]*dto.AvailableProviderResponse, 0, len(providers))
	for _, p := range providers {
		configFields := make([]dto.ConfigFieldResponse, 0, len(p.ConfigFields))
		for _, field := range p.ConfigFields {
			configFields = append(configFields, dto.ConfigFieldResponse{
				Key:            field.Key,
				Label:          field.Label,
				Description:    field.Description,
				Type:           field.Type,
				Required:       field.Required,
				Example:        field.Example,
				Placeholder:    field.Placeholder,
				ValidationRule: field.ValidationRule,
				HelpLink:       field.HelpLink,
				DefaultValue:   field.DefaultValue,
			})
		}

		result = append(result, &dto.AvailableProviderResponse{
			Code:              p.Code,
			Name:              p.Name,
			Type:              p.Type,
			Description:       p.Description,
			ConfigFields:      configFields,
			SupportsSend:      p.SupportsSend,
			SupportsBatchSend: p.SupportsBatchSend,
			SupportsCallback:  p.SupportsCallback,
			// 扩展信息
			Website:    p.Website,
			Icon:       p.Icon,
			DocsUrl:    p.DocsUrl,
			ConsoleUrl: p.ConsoleUrl,
			PricingUrl: p.PricingUrl,
			SortOrder:  p.SortOrder,
			Tags:       p.Tags,
			Regions:    p.Regions,
			Deprecated: p.Deprecated,
		})
	}

	return result, nil
}

// GetProviderConfigFields 获取指定服务商的配置字段定义
func (s *AdminProviderAccountService) GetProviderConfigFields(providerCode string) ([]dto.ConfigFieldResponse, error) {
	meta, err := registry.GetByCode(providerCode)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}

	result := make([]dto.ConfigFieldResponse, 0, len(meta.ConfigFields))
	for _, field := range meta.ConfigFields {
		result = append(result, dto.ConfigFieldResponse{
			Key:            field.Key,
			Label:          field.Label,
			Description:    field.Description,
			Type:           field.Type,
			Required:       field.Required,
			Example:        field.Example,
			Placeholder:    field.Placeholder,
			ValidationRule: field.ValidationRule,
			HelpLink:       field.HelpLink,
			DefaultValue:   field.DefaultValue,
		})
	}

	return result, nil
}

// CreateProviderAccount 创建服务商账号配置
func (s *AdminProviderAccountService) CreateProviderAccount(c *gin.Context, req *dto.CreateProviderAccountRequest) (*dto.ProviderAccountResponse, error) {
	logger := helper.GetHelper().GetLogger()

	// 验证服务商代码是否已注册
	meta, err := registry.GetByCode(req.ProviderCode)
	if err != nil {
		return nil, fmt.Errorf("invalid provider_code: %s not registered", req.ProviderCode)
	}

	status := int8(req.Status)
	if status == 0 {
		status = 1 // 默认启用
	}

	// 生成account_code
	accountCode, err := generateRandomKey(8)
	if err != nil {
		logger.Error("生成account_code失败")
		return nil, fmt.Errorf("failed to generate account_code: %w", err)
	}

	account := &model.ProviderAccount{
		AccountCode:  accountCode,
		AccountName:  req.Name,
		ProviderCode: req.ProviderCode,
		ProviderType: meta.Type, // 从注册信息中获取类型
		Status:       status,
		Remark:       req.Description,
	}

	// 设置配置
	if err := account.SetConfig(req.Config); err != nil {
		logger.Error("设置服务商配置失败")
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	accountDAO := dao.NewProviderAccountDAO()
	if err := accountDAO.Create(account); err != nil {
		logger.Error("创建服务商账号失败")
		return nil, err
	}

	logger.Info("服务商账号创建成功")

	config, _ := account.GetConfig()
	return &dto.ProviderAccountResponse{
		ID:           account.ID,
		AccountCode:  account.AccountCode,
		AccountName:  account.AccountName,
		ProviderCode: account.ProviderCode,
		ProviderName: meta.Name,
		ProviderType: account.ProviderType,
		Description:  account.Remark,
		Config:       config,
		Status:       int(account.Status),
		CallbackURL:  s.generateCallbackURL(c, account.ID, account.ProviderCode),
		CreatedAt:    account.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    account.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// GetProviderAccountList 获取服务商账号列表
func (s *AdminProviderAccountService) GetProviderAccountList(c *gin.Context, req *dto.ProviderAccountListRequest) (*dto.ProviderAccountListResponse, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	accountDAO := dao.NewProviderAccountDAO()
	accounts, total, err := accountDAO.List(page, pageSize, req.ProviderType, req.Status)
	if err != nil {
		return nil, fmt.Errorf("failed to query provider accounts: %w", err)
	}

	items := make([]*dto.ProviderAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		config, _ := account.GetConfig()

		// 从注册中心获取服务商名称
		providerName := account.ProviderCode
		if meta, err := registry.GetByCode(account.ProviderCode); err == nil {
			providerName = meta.Name
		}

		items = append(items, &dto.ProviderAccountResponse{
			ID:           account.ID,
			AccountCode:  account.AccountCode,
			AccountName:  account.AccountName,
			ProviderCode: account.ProviderCode,
			ProviderName: providerName,
			ProviderType: account.ProviderType,
			Description:  account.Remark,
			Config:       config,
			Status:       int(account.Status),
			CallbackURL:  s.generateCallbackURL(c, account.ID, account.ProviderCode),
			CreatedAt:    account.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    account.UpdatedAt.Format(time.RFC3339),
		})
	}

	return &dto.ProviderAccountListResponse{
		Total: int(total),
		Page:  page,
		Size:  pageSize,
		Items: items,
	}, nil
}

// GetProviderAccountByID 获取服务商账号详情
func (s *AdminProviderAccountService) GetProviderAccountByID(c *gin.Context, id uint) (*dto.ProviderAccountResponse, error) {
	accountDAO := dao.NewProviderAccountDAO()
	account, err := accountDAO.GetByID(id)
	if err != nil {
		return nil, err
	}

	config, _ := account.GetConfig()

	// 从注册中心获取服务商名称
	providerName := account.ProviderCode
	if meta, err := registry.GetByCode(account.ProviderCode); err == nil {
		providerName = meta.Name
	}

	return &dto.ProviderAccountResponse{
		ID:           account.ID,
		AccountCode:  account.AccountCode,
		AccountName:  account.AccountName,
		ProviderCode: account.ProviderCode,
		ProviderName: providerName,
		ProviderType: account.ProviderType,
		Description:  account.Remark,
		Config:       config,
		Status:       int(account.Status),
		CallbackURL:  s.generateCallbackURL(c, account.ID, account.ProviderCode),
		CreatedAt:    account.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    account.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// UpdateProviderAccount 更新服务商账号
func (s *AdminProviderAccountService) UpdateProviderAccount(id uint, req *dto.UpdateProviderAccountRequest) error {
	updates := make(map[string]interface{})

	if req.Name != "" {
		updates["account_name"] = req.Name
	}
	if req.Description != "" {
		updates["remark"] = req.Description
	}
	if req.Status > 0 {
		updates["status"] = int8(req.Status)
	}
	if len(req.Config) > 0 {
		// 需要先查询现有Account来处理配置
		accountDAO := dao.NewProviderAccountDAO()
		account, err := accountDAO.GetByID(id)
		if err != nil {
			return err
		}
		if err := account.SetConfig(req.Config); err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
		updates["config"] = account.Config
	}

	if len(updates) == 0 {
		return nil
	}

	accountDAO := dao.NewProviderAccountDAO()
	return accountDAO.UpdateFields(id, updates)
}

// DeleteProviderAccount 删除服务商账号
func (s *AdminProviderAccountService) DeleteProviderAccount(id uint) error {
	accountDAO := dao.NewProviderAccountDAO()
	return accountDAO.Delete(id)
}

// GetActiveProviderAccounts 获取活跃服务商账号列表
func (s *AdminProviderAccountService) GetActiveProviderAccounts(providerType string) ([]*dto.ActiveItem, error) {
	accountDAO := dao.NewProviderAccountDAO()
	accounts, err := accountDAO.GetActiveAccounts(providerType)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.ActiveItem, 0, len(accounts))
	for _, account := range accounts {
		// 从注册中心获取服务商名称
		providerName := account.ProviderCode
		if meta, err := registry.GetByCode(account.ProviderCode); err == nil {
			providerName = meta.Name
		}

		items = append(items, &dto.ActiveItem{
			ID:           account.ID,
			ProviderCode: account.AccountCode,
			ProviderName: account.AccountName,
			ProviderType: account.ProviderType,
			Name:         providerName,
		})
	}
	return items, nil
}

// TestProviderAccount 测试服务商账号配置
func (s *AdminProviderAccountService) TestProviderAccount(id uint, req *dto.TestProviderRequest) (*dto.TestProviderResponse, error) {
	// 1. 获取服务商账号配置
	accountDAO := dao.NewProviderAccountDAO()
	account, err := accountDAO.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("provider account not found: %w", err)
	}

	// 2. 构造发送请求
	task := &model.PushTask{
		MessageType: account.ProviderType,
	}

	// 根据类型填充接收者
	switch account.ProviderType {
	case "sms":
		if req.Phone == "" {
			return nil, fmt.Errorf("phone number is required for sms")
		}
		task.Receiver = req.Phone
		task.Content = req.Message
	case "email":
		if req.Email == "" {
			return nil, fmt.Errorf("email is required for email")
		}
		task.Receiver = req.Email
		task.Content = req.Message
		task.Title = "测试邮件"
	default:
		task.Content = req.Message
	}

	sendReq := &sender.SendRequest{
		Task:            task,
		ProviderAccount: account,
		Signature:       nil, // 测试时不加载签名，由服务商返回错误
	}

	// 3. 获取发送器（使用服务商代码）
	factory := sender.NewFactory()
	msgSender, err := factory.GetSender(account.ProviderCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender: %w", err)
	}

	// 4. 发送测试消息
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := msgSender.Send(ctx, sendReq)
	if err != nil {
		return &dto.TestProviderResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &dto.TestProviderResponse{
		Success: resp.Success,
		Message: resp.ErrorMessage,
	}, nil
}
