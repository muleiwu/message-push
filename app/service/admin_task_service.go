package service

import (
	"time"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/helper"
)

// AdminTaskService 管理后台任务服务
type AdminTaskService struct {
	pushTaskDAO        *dao.PushTaskDAO
	batchTaskDAO       *dao.PushBatchTaskDAO
	appDAO             *dao.ApplicationDAO
	providerAccountDAO *dao.ProviderAccountDAO
}

// NewAdminTaskService 创建服务
func NewAdminTaskService() *AdminTaskService {
	return &AdminTaskService{
		pushTaskDAO:        dao.NewPushTaskDAO(),
		batchTaskDAO:       dao.NewPushBatchTaskDAO(),
		appDAO:             dao.NewApplicationDAO(),
		providerAccountDAO: dao.NewProviderAccountDAO(),
	}
}

// GetPushTaskList 获取推送任务列表
func (s *AdminTaskService) GetPushTaskList(req *dto.PushTaskListRequest) (*dto.PushTaskListResponse, error) {
	// 构建过滤条件
	filters := make(map[string]interface{})
	if req.AppID != "" {
		filters["app_id"] = req.AppID
	}
	if req.Status != "" {
		filters["status"] = req.Status
	}
	if req.MessageType != "" {
		filters["message_type"] = req.MessageType
	}
	if req.TaskID != "" {
		filters["task_id"] = req.TaskID
	}
	if req.BatchID != "" {
		filters["batch_id"] = req.BatchID
	}
	if req.StartDate != "" {
		filters["start_date"] = req.StartDate
	}
	if req.EndDate != "" {
		filters["end_date"] = req.EndDate
	}

	tasks, total, err := s.pushTaskDAO.List(req.Page, req.PageSize, filters)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.PushTaskItem, 0, len(tasks))

	// 预加载缓存
	db := helper.GetHelper().GetDatabase()
	channelMap := make(map[uint]string)
	providerMap := make(map[uint]string)

	for _, task := range tasks {
		// 获取通道名称
		channelName := ""
		if task.ChannelID > 0 {
			if name, ok := channelMap[task.ChannelID]; ok {
				channelName = name
			} else {
				var channel model.Channel
				if err := db.First(&channel, task.ChannelID).Error; err == nil {
					channelName = channel.Name
					channelMap[task.ChannelID] = channelName
				}
			}
		}

		// 获取服务商账号名称
		providerAccountName := ""
		if task.ProviderAccountID != nil && *task.ProviderAccountID > 0 {
			if name, ok := providerMap[*task.ProviderAccountID]; ok {
				providerAccountName = name
			} else {
				account, err := s.providerAccountDAO.GetByID(*task.ProviderAccountID)
				if err == nil && account != nil {
					providerAccountName = account.AccountName
					providerMap[*task.ProviderAccountID] = providerAccountName
				}
			}
		}

		items = append(items, &dto.PushTaskItem{
			ID:                  task.ID,
			TaskID:              task.TaskID,
			AppID:               task.AppID,
			ChannelID:           task.ChannelID,
			ProviderAccountID:   task.ProviderAccountID,
			ProviderMsgID:       task.ProviderMsgID,
			MessageType:         task.MessageType,
			Receiver:            task.Receiver,
			Title:               task.Title,
			Content:             task.Content,
			TemplateCode:        task.TemplateCode,
			TemplateParams:      task.TemplateParams,
			Signature:           task.Signature,
			Status:              task.Status,
			CallbackStatus:      task.CallbackStatus,
			CallbackTime:        task.CallbackTime,
			RetryCount:          task.RetryCount,
			MaxRetry:            task.MaxRetry,
			ScheduledAt:         task.ScheduledAt,
			CreatedAt:           task.CreatedAt.Format(time.RFC3339),
			UpdatedAt:           task.UpdatedAt.Format(time.RFC3339),
			ChannelName:         channelName,
			ProviderAccountName: providerAccountName,
		})
	}

	return &dto.PushTaskListResponse{
		Total: total,
		Page:  req.Page,
		Size:  req.PageSize,
		Items: items,
	}, nil
}

// GetPushTask 获取单个推送任务详情
func (s *AdminTaskService) GetPushTask(id uint) (*dto.PushTaskItem, error) {
	task, err := s.pushTaskDAO.GetByID(id)
	if err != nil {
		return nil, err
	}

	return s.convertPushTaskToItem(task), nil
}

// GetPushBatchTaskList 获取批量任务列表
func (s *AdminTaskService) GetPushBatchTaskList(req *dto.PushBatchTaskListRequest) (*dto.PushBatchTaskListResponse, error) {
	// 构建过滤条件
	filters := make(map[string]interface{})
	if req.AppID != "" {
		filters["app_id"] = req.AppID
	}
	if req.Status != "" {
		filters["status"] = req.Status
	}
	if req.BatchID != "" {
		filters["batch_id"] = req.BatchID
	}
	if req.StartDate != "" {
		filters["start_date"] = req.StartDate
	}
	if req.EndDate != "" {
		filters["end_date"] = req.EndDate
	}

	batches, total, err := s.batchTaskDAO.List(req.Page, req.PageSize, filters)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.PushBatchTaskItem, 0, len(batches))

	for _, batch := range batches {
		items = append(items, s.convertBatchTaskToItem(batch))
	}

	return &dto.PushBatchTaskListResponse{
		Total: total,
		Page:  req.Page,
		Size:  req.PageSize,
		Items: items,
	}, nil
}

// GetPushBatchTask 获取单个批量任务详情
func (s *AdminTaskService) GetPushBatchTask(id uint) (*dto.PushBatchTaskItem, error) {
	batch, err := s.batchTaskDAO.GetByID(id)
	if err != nil {
		return nil, err
	}

	return s.convertBatchTaskToItem(batch), nil
}

// GetTasksByBatchID 根据批次ID获取该批次的所有任务
func (s *AdminTaskService) GetTasksByBatchID(batchID string, page, pageSize int) (*dto.PushTaskListResponse, error) {
	filters := map[string]interface{}{
		"batch_id": batchID,
	}

	tasks, total, err := s.pushTaskDAO.List(page, pageSize, filters)
	if err != nil {
		return nil, err
	}

	items := make([]*dto.PushTaskItem, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, s.convertPushTaskToItem(task))
	}

	return &dto.PushTaskListResponse{
		Total: total,
		Page:  page,
		Size:  pageSize,
		Items: items,
	}, nil
}

// convertPushTaskToItem 转换任务为DTO
func (s *AdminTaskService) convertPushTaskToItem(task *model.PushTask) *dto.PushTaskItem {
	db := helper.GetHelper().GetDatabase()

	// 获取通道名称
	channelName := ""
	if task.ChannelID > 0 {
		var channel model.Channel
		if err := db.First(&channel, task.ChannelID).Error; err == nil {
			channelName = channel.Name
		}
	}

	// 获取服务商账号名称
	providerAccountName := ""
	if task.ProviderAccountID != nil && *task.ProviderAccountID > 0 {
		account, err := s.providerAccountDAO.GetByID(*task.ProviderAccountID)
		if err == nil && account != nil {
			providerAccountName = account.AccountName
		}
	}

	return &dto.PushTaskItem{
		ID:                  task.ID,
		TaskID:              task.TaskID,
		AppID:               task.AppID,
		ChannelID:           task.ChannelID,
		ProviderAccountID:   task.ProviderAccountID,
		ProviderMsgID:       task.ProviderMsgID,
		MessageType:         task.MessageType,
		Receiver:            task.Receiver,
		Title:               task.Title,
		Content:             task.Content,
		TemplateCode:        task.TemplateCode,
		TemplateParams:      task.TemplateParams,
		Signature:           task.Signature,
		Status:              task.Status,
		CallbackStatus:      task.CallbackStatus,
		CallbackTime:        task.CallbackTime,
		RetryCount:          task.RetryCount,
		MaxRetry:            task.MaxRetry,
		ScheduledAt:         task.ScheduledAt,
		CreatedAt:           task.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           task.UpdatedAt.Format(time.RFC3339),
		ChannelName:         channelName,
		ProviderAccountName: providerAccountName,
	}
}

// convertBatchTaskToItem 转换批量任务为DTO
func (s *AdminTaskService) convertBatchTaskToItem(batch *model.PushBatchTask) *dto.PushBatchTaskItem {
	// 计算完成率
	completionRate := 0.0
	if batch.TotalCount > 0 {
		completionRate = float64(batch.SuccessCount+batch.FailedCount) / float64(batch.TotalCount) * 100
	}

	return &dto.PushBatchTaskItem{
		ID:             batch.ID,
		BatchID:        batch.BatchID,
		AppID:          batch.AppID,
		TotalCount:     batch.TotalCount,
		SuccessCount:   batch.SuccessCount,
		FailedCount:    batch.FailedCount,
		PendingCount:   batch.PendingCount,
		Status:         batch.Status,
		CreatedAt:      batch.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      batch.UpdatedAt.Format(time.RFC3339),
		CompletionRate: completionRate,
	}
}
