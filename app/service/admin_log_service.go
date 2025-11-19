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
	providerChannelDAO *dao.ProviderChannelDAO
	providerDAO        *dao.ProviderDAO
}

// NewAdminLogService 创建服务
func NewAdminLogService() *AdminLogService {
	return &AdminLogService{
		logDAO:             dao.NewPushLogDAO(),
		appDAO:             dao.NewApplicationDAO(),
		providerChannelDAO: dao.NewProviderChannelDAO(),
		providerDAO:        dao.NewProviderDAO(),
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

		// 获取服务商名称
		providerName, ok := providerMap[log.ProviderChannelID]
		if !ok {
			pc, err := s.providerChannelDAO.GetByID(log.ProviderChannelID)
			if err == nil && pc != nil {
				// 关联服务商表
				provider, err := s.providerDAO.GetByID(pc.ProviderID)
				if err == nil && provider != nil {
					providerName = provider.ProviderName
				} else {
					providerName = "未知服务商"
				}
			} else {
				providerName = "未知通道"
			}
			providerMap[log.ProviderChannelID] = providerName
		}

		items = append(items, &dto.LogItem{
			ID:                log.ID,
			TaskID:            log.TaskID,
			AppID:             log.AppID,
			AppName:           appName,
			ProviderChannelID: log.ProviderChannelID,
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
	pc, err := s.providerChannelDAO.GetByID(log.ProviderChannelID)
	if err == nil && pc != nil {
		provider, err := s.providerDAO.GetByID(pc.ProviderID)
		if err == nil && provider != nil {
			providerName = provider.ProviderName
		}
	}

	return &dto.LogItem{
		ID:                log.ID,
		TaskID:            log.TaskID,
		AppID:             log.AppID,
		AppName:           appName,
		ProviderChannelID: log.ProviderChannelID,
		ProviderName:      providerName,
		RequestData:       log.RequestData,
		ResponseData:      log.ResponseData,
		Status:            log.Status,
		ErrorMessage:      log.ErrorMessage,
		CostTime:          log.CostTime,
		CreatedAt:         log.CreatedAt.Format(time.RFC3339),
	}, nil
}
