package service

import (
	"encoding/json"
	"strings"

	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/internal/timeutil"
	"cnb.cool/mliev/push/message-push/modules/template"
	"gorm.io/gorm"
)

// AdminMessageDetailService resolves saved attempts or explicitly labelled legacy
// previews. It never calls the selector, sender or remote provider APIs.
type AdminMessageDetailService struct {
	db       *gorm.DB
	renderer template.Renderer
}

func NewAdminMessageDetailService(db *gorm.DB, renderer template.Renderer) *AdminMessageDetailService {
	return &AdminMessageDetailService{db: db, renderer: renderer}
}

func unavailableMessageDetail(logID uint, reason string) *dto.MessageDetail {
	return &dto.MessageDetail{
		Source: dto.MessageDetailUnavailable,
		LogID:  logID,
		SendSnapshot: model.SendSnapshot{MessageContent: model.MessageContent{
			ContentType:       "text",
			UnavailableReason: reason,
		}},
	}
}

// Latest consumes logs in descending ID order. NULL snapshot logs after rollout
// are callback actions, so they cannot replace the most recent send attempt.
func (s *AdminMessageDetailService) Latest(task *model.PushTask, logs []*model.PushLog) *dto.MessageDetail {
	for _, log := range logs {
		if log.SendSnapshot != nil {
			return savedMessageDetail(log)
		}
	}
	if len(logs) == 0 {
		return unavailableMessageDetail(0, "尚未生成发送内容")
	}
	callbacks, err := s.callbackRequests(task)
	if err != nil {
		return unavailableMessageDetail(0, "读取历史日志来源失败")
	}
	for _, log := range logs {
		if !callbacks[strings.TrimSpace(log.RequestData)] {
			return s.legacy(task, log)
		}
	}
	return unavailableMessageDetail(0, "未找到发送尝试记录")
}

func (s *AdminMessageDetailService) ForLogs(task *model.PushTask, logs []*model.PushLog) map[uint]*dto.MessageDetail {
	firstSnapshotID := uint(0)
	for _, log := range logs {
		if log.SendSnapshot != nil && (firstSnapshotID == 0 || log.ID < firstSnapshotID) {
			firstSnapshotID = log.ID
		}
	}
	result := make(map[uint]*dto.MessageDetail, len(logs))
	var callbackRequests map[string]bool
	var callbackErr error
	callbacksLoaded := false
	for _, log := range logs {
		switch {
		case log.SendSnapshot != nil:
			result[log.ID] = savedMessageDetail(log)
		case firstSnapshotID != 0 && log.ID > firstSnapshotID:
			result[log.ID] = unavailableMessageDetail(log.ID, "该日志未产生新的发送内容")
		default:
			if !callbacksLoaded {
				callbackRequests, callbackErr = s.callbackRequests(task)
				callbacksLoaded = true
			}
			switch {
			case callbackErr != nil:
				result[log.ID] = unavailableMessageDetail(log.ID, "读取历史日志来源失败")
			case callbackRequests[strings.TrimSpace(log.RequestData)]:
				result[log.ID] = unavailableMessageDetail(log.ID, "该日志为回调处理记录，未产生新的发送内容")
			default:
				result[log.ID] = s.legacy(task, log)
			}
		}
	}
	return result
}

// Legacy rule-action logs did not record their scene. Callback failure actions
// copy the callback body to request_data, so the callback audit can identify them
// without mistaking them for a new outbound attempt.
func (s *AdminMessageDetailService) callbackRequests(task *model.PushTask) (map[string]bool, error) {
	requests := make(map[string]bool)
	if task == nil {
		return requests, nil
	}
	var bodies []string
	if err := s.db.Model(&model.CallbackLog{}).Where("task_id = ?", task.TaskID).Pluck("raw_data", &bodies).Error; err != nil {
		return nil, err
	}
	for _, body := range bodies {
		body = strings.TrimSpace(body)
		if body != "" && body != "{}" && body != "null" {
			requests[body] = true
		}
	}
	return requests, nil
}

func savedMessageDetail(log *model.PushLog) *dto.MessageDetail {
	var snapshot model.SendSnapshot
	if err := json.Unmarshal([]byte(*log.SendSnapshot), &snapshot); err != nil || snapshot.Version != model.SendSnapshotVersion {
		return unavailableMessageDetail(log.ID, "发送快照损坏或版本不受支持")
	}
	source := dto.MessageDetailSnapshot
	if snapshot.UnavailableReason != "" {
		source = dto.MessageDetailUnavailable
	}
	return &dto.MessageDetail{Source: source, LogID: log.ID, SendSnapshot: snapshot}
}

func (s *AdminMessageDetailService) legacy(task *model.PushTask, log *model.PushLog) *dto.MessageDetail {
	if task == nil || log.ProviderAccountID == 0 {
		return unavailableMessageDetail(log.ID, "无法确定该次发送使用的通道或供应商")
	}
	code, valid := legacyTemplateCode(log.RequestData)
	if !valid {
		return unavailableMessageDetail(log.ID, "历史请求数据无效，无法定位发送模板")
	}
	if code == "" {
		code = task.TemplateCode
	}
	var bindings []*model.ChannelTemplateBinding
	err := s.db.Where("channel_id = ? AND provider_id = ?", task.ChannelID, log.ProviderAccountID).
		Preload("ProviderTemplate").
		Preload("ProviderAccount", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "account_name", "provider_code")
		}).Find(&bindings).Error
	if err != nil {
		return unavailableMessageDetail(log.ID, "读取当前模板配置失败")
	}
	var candidates []*model.ChannelTemplateBinding
	for _, binding := range bindings {
		if binding.ProviderTemplate != nil && binding.ProviderTemplate.ProviderID == log.ProviderAccountID &&
			(code == "" || binding.ProviderTemplate.TemplateCode == code) {
			candidates = append(candidates, binding)
		}
	}
	if len(candidates) == 0 {
		return unavailableMessageDetail(log.ID, "原发送模板或绑定已不存在，无法生成当前配置预览")
	}
	if len(candidates) != 1 {
		return unavailableMessageDetail(log.ID, "存在多个候选模板，无法唯一确定原发送模板")
	}
	binding := candidates[0]
	detail := &dto.MessageDetail{
		Source: dto.MessageDetailCurrentConfig,
		LogID:  log.ID,
		SendSnapshot: model.SendSnapshot{
			MessageContent:    template.PrepareContent(s.renderer, task.TemplateParams, binding),
			ProviderAccountID: log.ProviderAccountID,
			SignatureAlias:    task.Signature,
			CapturedAt:        timeutil.FormatRFC3339(log.CreatedAt),
		},
	}
	if binding.ProviderAccount != nil {
		detail.ProviderName = binding.ProviderAccount.AccountName
		detail.ProviderCode = binding.ProviderAccount.ProviderCode
	}
	if task.Signature != "" {
		signature, err := dao.NewChannelSignatureMappingDAO(s.db).
			GetByChannelIDAndSignatureName(task.ChannelID, task.Signature, log.ProviderAccountID)
		if err == nil {
			detail.SignatureValue = signature.SignatureCode
		}
	}
	if detail.UnavailableReason != "" {
		detail.Source = dto.MessageDetailUnavailable
	}
	return detail
}

// These are the audited request keys used by the existing SMS senders. Do not
// interpret arbitrary "content" values as rendered text (some contain slot JSON).
func legacyTemplateCode(raw string) (string, bool) {
	if strings.TrimSpace(raw) == "" {
		return "", true
	}
	var request map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &request); err != nil {
		return "", false
	}
	for _, key := range []string{"template_code", "template_id", "templateid"} {
		value, exists := request[key]
		if !exists {
			continue
		}
		var code string
		if err := json.Unmarshal(value, &code); err == nil {
			return code, true
		}
		var number json.Number
		if err := json.Unmarshal(value, &number); err == nil && number != "" {
			return number.String(), true
		}
		return "", false
	}
	return "", true
}
