package service

import (
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
)

// AdminLogService 管理后台日志服务
type AdminLogService struct {
	logDAO             *dao.PushLogDAO
	appDAO             *dao.ApplicationDAO
	providerAccountDAO *dao.ProviderAccountDAO
}

// NewAdminLogService 创建服务
func NewAdminLogService() *AdminLogService {
	return &AdminLogService{
		logDAO:             dao.NewPushLogDAO(),
		appDAO:             dao.NewApplicationDAO(),
		providerAccountDAO: dao.NewProviderAccountDAO(),
	}
}

// GetLogList 获取日志列表
func (s *AdminLogService) GetLogList(req *dto.LogListRequest) (*dto.LogListResponse, error) {
	logs, total, err := s.logDAO.List(req)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.LogItem, 0, len(logs))

	// 预加载缓存（简单优化，避免循环查库）
	appMap := make(map[string]string)
	providerMap := make(map[uint]string)

	for _, log := range logs {
		// 获取应用名称
		appName, ok := appMap[log.AppID]
		if !ok {
			app, err := s.appDAO.GetByAppID(log.AppID)
			if err == nil && app != nil {
				appName = app.AppName
			} else {
				appName = "未知应用"
			}
			appMap[log.AppID] = appName
		}

		// 获取服务商账号名称
		providerName, ok := providerMap[log.ProviderAccountID]
		if !ok {
			account, err := s.providerAccountDAO.GetByID(log.ProviderAccountID)
			if err == nil && account != nil {
				providerName = account.AccountName
			} else {
				providerName = "未知服务商"
			}
			providerMap[log.ProviderAccountID] = providerName
		}

		items = append(items, &dto.LogItem{
			ID:                log.ID,
			TaskID:            log.TaskID,
			AppID:             log.AppID,
			AppName:           appName,
			ProviderAccountID: log.ProviderAccountID,
			ProviderName:      providerName,
			RequestData:       log.RequestData,
			ResponseData:      log.ResponseData,
			Status:            log.Status,
			ErrorMessage:      log.ErrorMessage,
			CostTime:          log.CostTime,
			CreatedAt:         log.CreatedAt.Format(time.RFC3339),
		})
	}

	return &dto.LogListResponse{
		Total: total,
		Page:  req.Page,
		Size:  req.PageSize,
		Items: items,
	}, nil
}

// GetLog 获取日志详情
func (s *AdminLogService) GetLog(id uint) (*dto.LogItem, error) {
	log, err := s.logDAO.GetByID(id)
	if err != nil {
		return nil, err
	}

	// 获取关联信息
	appName := ""
	app, err := s.appDAO.GetByAppID(log.AppID)
	if err == nil && app != nil {
		appName = app.AppName
	}

	providerName := ""
	account, err := s.providerAccountDAO.GetByID(log.ProviderAccountID)
	if err == nil && account != nil {
		providerName = account.AccountName
	}

	return &dto.LogItem{
		ID:                log.ID,
		TaskID:            log.TaskID,
		AppID:             log.AppID,
		AppName:           appName,
		ProviderAccountID: log.ProviderAccountID,
		ProviderName:      providerName,
		RequestData:       log.RequestData,
		ResponseData:      log.ResponseData,
		Status:            log.Status,
		ErrorMessage:      log.ErrorMessage,
		CostTime:          log.CostTime,
		CreatedAt:         log.CreatedAt.Format(time.RFC3339),
	}, nil
}
