package service

import (
	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/timeutil"
	"cnb.cool/mliev/push/message-push/modules/template"
)

// AdminTaskService 管理后台任务服务
type AdminTaskService struct {
	messageDetails *AdminMessageDetailService
	pushTaskDAO    *dao.PushTaskDAO
	batchTaskDAO   *dao.PushBatchTaskDAO
	appDAO         *dao.ApplicationDAO
	pushLogDAO     *dao.PushLogDAO
}

// NewAdminTaskService 创建服务
func NewAdminTaskService() *AdminTaskService {
	return &AdminTaskService{
		messageDetails: NewAdminMessageDetailService(helper.GetDatabase(), template.GetRenderer()),
		pushTaskDAO:    dao.NewPushTaskDAO(),
		batchTaskDAO:   dao.NewPushBatchTaskDAO(),
		appDAO:         dao.NewApplicationDAO(),
		pushLogDAO:     dao.NewPushLogDAO(),
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
	if req.Receiver != "" {
		filters["receiver"] = req.Receiver
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

	for _, task := range tasks {
		items = append(items, s.convertPushTaskToItem(task))
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

	item := s.convertPushTaskToItem(task)
	logs, err := s.pushLogDAO.GetByTaskID(task.TaskID)
	if err != nil {
		return nil, err
	}
	item.MessageDetail = s.messageDetails.Latest(task, logs)
	return item, nil
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
	channelName := ""
	if task.Channel != nil {
		channelName = task.Channel.Name
	}

	providerAccountName := ""
	if task.ProviderAccount != nil {
		providerAccountName = task.ProviderAccount.AccountName
	}

	// 从最新的 push_log 获取 ProviderMsgID
	providerMsgID := ""
	if log, err := s.pushLogDAO.GetLatestSummary(task.TaskID); err == nil {
		providerMsgID = log.ProviderMsgID
	}

	return &dto.PushTaskItem{
		ID:                  task.ID,
		TaskID:              task.TaskID,
		AppID:               task.AppID,
		ChannelID:           task.ChannelID,
		ProviderAccountID:   task.ProviderAccountID,
		ProviderMsgID:       providerMsgID,
		MessageType:         task.MessageType,
		Receiver:            task.Receiver,
		Content:             "", // 保留旧字段兼容性；详情正文由 MessageDetail 提供
		TemplateCode:        task.TemplateCode,
		TemplateParams:      task.TemplateParams,
		Signature:           task.Signature,
		Status:              task.Status,
		CallbackStatus:      task.CallbackStatus,
		CallbackTime:        timeutil.NormalizePtr(task.CallbackTime),
		RetryCount:          task.RetryCount,
		MaxRetry:            task.MaxRetry,
		ScheduledAt:         timeutil.NormalizePtr(task.ScheduledAt),
		CreatedAt:           timeutil.FormatRFC3339(task.CreatedAt),
		UpdatedAt:           timeutil.FormatRFC3339(task.UpdatedAt),
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
		CreatedAt:      timeutil.FormatRFC3339(batch.CreatedAt),
		UpdatedAt:      timeutil.FormatRFC3339(batch.UpdatedAt),
		CompletionRate: completionRate,
	}
}
