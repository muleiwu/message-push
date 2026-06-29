package service

import (
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
)

// AdminCallbackService 管理后台服务商回调记录服务（统一下行回执与上行短信）
type AdminCallbackService struct {
	callbackLogDAO *dao.CallbackLogDAO
	appDAO         *dao.ApplicationDAO
}

// NewAdminCallbackService 创建服务
func NewAdminCallbackService() *AdminCallbackService {
	return &AdminCallbackService{
		callbackLogDAO: dao.NewCallbackLogDAO(),
		appDAO:         dao.NewApplicationDAO(),
	}
}

// GetCallbackList 获取回调记录列表
func (s *AdminCallbackService) GetCallbackList(req *dto.CallbackListRequest) (*dto.CallbackListResponse, error) {
	logs, total, err := s.callbackLogDAO.List(req)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.CallbackItem, 0, len(logs))

	// 预加载应用名缓存，避免循环查库
	appMap := make(map[string]string)

	for _, log := range logs {
		appName := ""
		if log.AppID != "" {
			name, ok := appMap[log.AppID]
			if !ok {
				app, err := s.appDAO.GetByAppID(log.AppID)
				if err == nil && app != nil {
					name = app.AppName
				} else {
					name = "未知应用"
				}
				appMap[log.AppID] = name
			}
			appName = name
		}

		items = append(items, &dto.CallbackItem{
			ID:             log.ID,
			Type:           log.Type,
			TaskID:         log.TaskID,
			AppID:          log.AppID,
			AppName:        appName,
			ProviderCode:   log.ProviderCode,
			ProviderID:     log.ProviderID,
			Mobile:         log.Mobile,
			Content:        log.Content,
			CallbackStatus: log.CallbackStatus,
			ErrorCode:      log.ErrorCode,
			ErrorMessage:   log.ErrorMessage,
			RawData:        log.RawData,
			CreatedAt:      log.CreatedAt.Format(time.RFC3339),
		})
	}

	return &dto.CallbackListResponse{
		Total: total,
		Page:  req.Page,
		Size:  req.PageSize,
		Items: items,
	}, nil
}
