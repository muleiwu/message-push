package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/channel"
	"cnb.cool/mliev/push/message-push/modules/delivery/infrastructure/lock"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrResourceConflict = errors.New("资源已变化，请重新查询并确认")
var resourceAccountLocks sync.Map

type ResourceImpact struct {
	ChannelID uint   `json:"channel_id"`
	Name      string `json:"name"`
}

type ResourcePreviewItem struct {
	Remote           domain.RemoteResource            `json:"remote"`
	Parsed           *domain.ParsedTemplate           `json:"parsed,omitempty"`
	LocalID          uint                             `json:"local_id"`
	LocalName        string                           `json:"local_name"`
	LocalContent     string                           `json:"local_content"`
	LocalVariables   []string                         `json:"local_variables"`
	LocalStatus      int8                             `json:"local_status"`
	LocalAuditStatus *int8                            `json:"local_audit_status"`
	Deleted          bool                             `json:"deleted"`
	Version          string                           `json:"version"`
	Error            string                           `json:"error,omitempty"`
	IdentityConflict bool                             `json:"identity_conflict"`
	Impacts          []ResourceImpact                 `json:"impacts"`
	OperationErrors  map[domain.ResourceAction]string `json:"operation_errors,omitempty"`
	template         *model.ProviderTemplate
	signature        *model.ProviderSignature
}

type ResourceSelection struct {
	ID      string `json:"id"`
	Action  string `json:"action"`
	Version string `json:"version"`
}
type ResourceImportRequest struct {
	Kind       domain.ResourceKind `json:"kind"`
	Selections []ResourceSelection `json:"selections"`
}
type ResourceImportResult struct {
	Created  int `json:"created"`
	Updated  int `json:"updated"`
	Restored int `json:"restored"`
	Skipped  int `json:"skipped"`
}
type ResourceMutationRequest struct {
	domain.ResourceInput
	Version       string `json:"version"`
	ConfirmImpact bool   `json:"confirm_impact"`
}
type ResourceMutationResult struct {
	RequestID   string `json:"request_id,omitempty"`
	RemoteID    string `json:"remote_id"`
	LocalID     uint   `json:"local_id"`
	PendingSync bool   `json:"pending_sync"`
	Warning     string `json:"warning,omitempty"`
}

// AdminProviderResourceService orchestrates normalized resource operations.
// HTTP contracts and variable grammars are resolved exclusively from registration.
type AdminProviderResourceService struct {
	db         *gorm.DB
	lookup     func(string) (*domain.ProviderMeta, error)
	acquire    func(context.Context, uint) (func(), error)
	invalidate func(uint)
}

func NewAdminProviderResourceService() *AdminProviderResourceService {
	s := NewAdminProviderResourceServiceWithDB(helper.GetDatabase())
	s.acquire = func(ctx context.Context, id uint) (func(), error) {
		l := lock.NewRedisLock(helper.GetRedis(), fmt.Sprintf("provider-resources:%d", id), 2*time.Minute)
		ok, err := l.TryLock(ctx)
		if err != nil {
			return nil, fmt.Errorf("无法获取资源操作锁")
		}
		if !ok {
			return nil, fmt.Errorf("该账号有资源操作正在执行，请稍后重试")
		}
		return func() {
			unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = l.Unlock(unlockCtx)
		}, nil
	}
	s.invalidate = func(id uint) { channel.GetSelector().InvalidateCacheForBinding(id) }
	return s
}

func NewAdminProviderResourceServiceWithDB(db *gorm.DB) *AdminProviderResourceService {
	return &AdminProviderResourceService{db: db, lookup: domain.GetByCode, acquire: func(ctx context.Context, id uint) (func(), error) {
		entry, _ := resourceAccountLocks.LoadOrStore(id, make(chan struct{}, 1))
		ch := entry.(chan struct{})
		select {
		case ch <- struct{}{}:
			return func() { <-ch }, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, invalidate: func(uint) {}}
}

func (s *AdminProviderResourceService) resolve(ctx context.Context, id uint, kind domain.ResourceKind) (*model.ProviderAccount, *domain.ResourceDefinition, error) {
	if kind != domain.ResourceTemplates && kind != domain.ResourceSignatures {
		return nil, nil, domain.ErrResourceUnsupported
	}
	var account model.ProviderAccount
	if err := s.db.WithContext(ctx).First(&account, id).Error; err != nil {
		return nil, nil, fmt.Errorf("供应商账号不存在")
	}
	meta, err := s.lookup(account.ProviderCode)
	if err != nil {
		return nil, nil, err
	}
	definition := meta.Resources[kind]
	if definition == nil {
		return nil, nil, domain.ErrResourceUnsupported
	}
	return &account, definition, nil
}

func (s *AdminProviderResourceService) Parse(ctx context.Context, accountID uint, content string) (*domain.ParsedTemplate, error) {
	var account model.ProviderAccount
	if err := s.db.WithContext(ctx).First(&account, accountID).Error; err != nil {
		return nil, err
	}
	meta, err := s.lookup(account.ProviderCode)
	if err != nil {
		return nil, err
	}
	if meta.TemplateCodec == nil {
		return nil, domain.ErrResourceUnsupported
	}
	return meta.TemplateCodec.Parse(content, &account)
}

func (s *AdminProviderResourceService) Preview(ctx context.Context, accountID uint, kind domain.ResourceKind, id string) ([]*ResourcePreviewItem, error) {
	account, definition, err := s.resolve(ctx, accountID, kind)
	if err != nil {
		return nil, err
	}
	resources, err := definition.Execute(ctx, account, domain.ResourceQuery, domain.ResourceInput{ID: id})
	if err != nil {
		return nil, err
	}
	items := make([]*ResourcePreviewItem, 0, len(resources))
	for _, resource := range resources {
		item, err := s.previewItem(s.db.WithContext(ctx), accountID, kind, definition, resource)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// Workspace prepares operation targets. A provider without Query exposes only
// its already linked local mirrors; it never claims to have queried upstream.
func (s *AdminProviderResourceService) Workspace(ctx context.Context, accountID uint, kind domain.ResourceKind) ([]*ResourcePreviewItem, error) {
	_, definition, err := s.resolve(ctx, accountID, kind)
	if err != nil {
		return nil, err
	}
	if definition.Capability().Query {
		return s.Preview(ctx, accountID, kind, "")
	}
	resources, err := s.localResourceFacts(ctx, accountID, kind, "")
	if err != nil {
		return nil, err
	}
	items := make([]*ResourcePreviewItem, 0, len(resources))
	for _, resource := range resources {
		item, err := s.previewItem(s.db.WithContext(ctx), accountID, kind, definition, resource)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *AdminProviderResourceService) localResourceFacts(ctx context.Context, accountID uint, kind domain.ResourceKind, id string) ([]domain.RemoteResource, error) {
	db := s.db.WithContext(ctx)
	identity := "remote_id"
	if kind == domain.ResourceTemplates {
		identity = "template_code"
	}
	db = db.Where(identity + " <> ''")
	if id != "" {
		db = db.Where(identity+" = ?", id)
	}
	result := []domain.RemoteResource{}
	if kind == domain.ResourceTemplates {
		var rows []model.ProviderTemplate
		if err := db.Where("provider_id = ?", accountID).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			status := int8(0)
			if row.AuditStatus != nil {
				status = *row.AuditStatus
			}
			result = append(result, domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: row.TemplateCode, Name: row.RemoteName, Content: row.TemplateContent, Category: row.Category, Description: row.RemoteDescription}, AuditStatus: status, AuditReply: row.AuditReply})
		}
	} else {
		var rows []model.ProviderSignature
		if err := db.Where("provider_account_id = ?", accountID).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			status := int8(0)
			if row.AuditStatus != nil {
				status = *row.AuditStatus
			}
			result = append(result, domain.RemoteResource{ResourceInput: domain.ResourceInput{ID: row.RemoteID, Content: row.SignatureCode, Description: row.RemoteDescription}, AuditStatus: status, AuditReply: row.AuditReply})
		}
	}
	return result, nil
}

func (s *AdminProviderResourceService) previewItem(db *gorm.DB, accountID uint, kind domain.ResourceKind, definition *domain.ResourceDefinition, resource domain.RemoteResource) (*ResourcePreviewItem, error) {
	item := &ResourcePreviewItem{Remote: resource, LocalVariables: []string{}, Impacts: []ResourceImpact{}}
	if utf8.RuneCountInString(resource.ID) > 100 || resource.ID == "" {
		item.Error = "供应商资源 ID 无效"
	}
	var local any
	if kind == domain.ResourceTemplates {
		var candidates []model.ProviderTemplate
		if err := db.Unscoped().Where("provider_id = ? AND template_code = ?", accountID, resource.ID).Order("id DESC").Find(&candidates).Error; err != nil {
			return nil, err
		}
		candidates = preferLiveResources(candidates, func(t model.ProviderTemplate) bool { return t.DeletedAt.Valid })
		if len(candidates) > 1 {
			item.IdentityConflict = true
			item.Error = "本地存在多个相同模板代码，请先处理重复记录"
		}
		if len(candidates) == 1 {
			t := &candidates[0]
			item.template = t
			local = t
			item.LocalID = t.ID
			item.LocalName = t.TemplateName
			item.LocalContent = t.TemplateContent
			item.LocalStatus = t.Status
			item.LocalAuditStatus = t.AuditStatus
			item.Deleted = t.DeletedAt.Valid
			var account model.ProviderAccount
			if err := db.First(&account, accountID).Error; err != nil {
				return nil, err
			}
			t.ProviderAccount = &account
			item.LocalVariables, _ = domain.ProviderTemplateVariables(t)
			// List APIs may omit the name/description; retain known provider facts.
			if item.Remote.Name == "" {
				item.Remote.Name = t.RemoteName
			}
			if item.Remote.Description == "" {
				item.Remote.Description = t.RemoteDescription
			}
		}
		var account model.ProviderAccount
		if err := db.First(&account, accountID).Error; err != nil {
			return nil, err
		}
		meta, err := s.lookup(account.ProviderCode)
		if err != nil {
			return nil, err
		}
		if meta.TemplateCodec == nil {
			return nil, domain.ErrResourceUnsupported
		}
		parsed, err := meta.TemplateCodec.Parse(resource.Content, &account)
		if err != nil {
			item.Error = err.Error()
		} else {
			item.Parsed = parsed
		}
		if item.LocalID > 0 {
			if err := db.Table("channel_template_bindings AS b").Select("DISTINCT c.id AS channel_id, c.name").Joins("JOIN channels c ON c.id = b.channel_id").Where("b.provider_template_id = ? AND b.deleted_at IS NULL AND c.deleted_at IS NULL", item.LocalID).Scan(&item.Impacts).Error; err != nil {
				return nil, err
			}
		}
	} else {
		if utf8.RuneCountInString(resource.Content) > 200 {
			item.Error = "供应商签名超过 200 字符"
		}
		var candidates []model.ProviderSignature
		if err := db.Unscoped().Where("provider_account_id = ? AND remote_id = ?", accountID, resource.ID).Order("id DESC").Find(&candidates).Error; err != nil {
			return nil, err
		}
		if len(candidates) == 0 {
			aliases := []string{resource.Content}
			if definition.MatchAliases != nil {
				aliases = definition.MatchAliases(resource.Content)
			}
			if err := db.Unscoped().Where("provider_account_id = ? AND signature_code IN ?", accountID, aliases).Where("remote_id = '' OR remote_id = ? OR deleted_at IS NULL", resource.ID).Order("id DESC").Find(&candidates).Error; err != nil {
				return nil, err
			}
		}
		candidates = preferLiveResources(candidates, func(t model.ProviderSignature) bool { return t.DeletedAt.Valid })
		if len(candidates) > 1 {
			item.IdentityConflict = true
			item.Error = "签名匹配到多个本地资源，请先处理重复记录"
		}
		if len(candidates) == 1 {
			t := &candidates[0]
			item.signature = t
			local = t
			item.LocalID = t.ID
			item.LocalName = t.SignatureName
			item.LocalContent = t.SignatureCode
			item.LocalStatus = t.Status
			item.LocalAuditStatus = t.AuditStatus
			item.Deleted = t.DeletedAt.Valid
			if t.RemoteID != "" && t.RemoteID != resource.ID {
				item.IdentityConflict = true
				item.Error = "该签名已关联另一远端 ID，请先处理关联冲突"
			}
			if item.Remote.Description == "" {
				item.Remote.Description = t.RemoteDescription
			}
		}
		if item.LocalID > 0 {
			if err := db.Table("channel_signature_mappings AS m").Select("DISTINCT c.id AS channel_id, c.name").Joins("JOIN channels c ON c.id = m.channel_id").Where("m.provider_signature_id = ? AND m.deleted_at IS NULL AND c.deleted_at IS NULL", item.LocalID).Scan(&item.Impacts).Error; err != nil {
				return nil, err
			}
		}
	}
	item.OperationErrors = definition.OperationErrors(resource)
	reconcileResourceReview(item, definition)
	// Include local identity/state and affected references in optimistic confirmation.
	raw, _ := json.Marshal([]any{accountID, kind, resource, local, item.Impacts})
	digest := sha256.Sum256(raw)
	item.Version = hex.EncodeToString(digest[:])
	return item, nil
}

// Prefer current records over historical copies. With only deleted copies,
// restore the most recently created one (queries order by descending ID).
func preferLiveResources[T any](rows []T, deleted func(T) bool) []T {
	live := make([]T, 0, len(rows))
	for _, row := range rows {
		if !deleted(row) {
			live = append(live, row)
		}
	}
	if len(live) > 0 {
		return live
	}
	if len(rows) > 0 {
		return rows[:1]
	}
	return rows
}

func (s *AdminProviderResourceService) Import(ctx context.Context, accountID uint, req ResourceImportRequest) (*ResourceImportResult, error) {
	if len(req.Selections) == 0 || len(req.Selections) > 1000 {
		return nil, fmt.Errorf("请选择 1 至 1000 个资源")
	}
	unlock, err := s.acquire(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	account, definition, err := s.resolve(ctx, accountID, req.Kind)
	if err != nil {
		return nil, err
	}
	resources, err := definition.Execute(ctx, account, domain.ResourceQuery, domain.ResourceInput{})
	if err != nil {
		return nil, err
	}
	byID := map[string]domain.RemoteResource{}
	for _, r := range resources {
		byID[r.ID] = r
	}
	result := &ResourceImportResult{}
	affected := map[uint]bool{}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seen := map[string]bool{}
		for _, selected := range req.Selections {
			if seen[selected.ID] {
				return fmt.Errorf("资源选择重复")
			}
			seen[selected.ID] = true
			if selected.Action == "skip" {
				result.Skipped++
				continue
			}
			remote, ok := byID[selected.ID]
			if !ok {
				return ErrResourceConflict
			}
			item, err := s.previewItem(tx, accountID, req.Kind, definition, remote)
			if err != nil {
				return err
			}
			if item.Error != "" {
				return fmt.Errorf("%s", item.Error)
			}
			if selected.Version == "" || item.Version != selected.Version {
				return ErrResourceConflict
			}
			switch selected.Action {
			case "create":
				if item.LocalID != 0 {
					return ErrResourceConflict
				}
				result.Created++
			case "update":
				if item.LocalID == 0 || item.Deleted {
					return ErrResourceConflict
				}
				result.Updated++
			case "restore":
				if !item.Deleted {
					return ErrResourceConflict
				}
				result.Restored++
			default:
				return fmt.Errorf("请明确选择新增、更新、恢复或跳过")
			}
			if _, err = s.saveMirror(tx, accountID, req.Kind, item, selected.Action == "restore"); err != nil {
				return err
			}
			for _, impact := range item.Impacts {
				affected[impact.ChannelID] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for id := range affected {
		s.invalidate(id)
	}
	return result, nil
}

func (s *AdminProviderResourceService) saveMirror(tx *gorm.DB, accountID uint, kind domain.ResourceKind, item *ResourcePreviewItem, restore bool) (uint, error) {
	now := time.Now().UTC()
	status := item.Remote.AuditStatus
	state := model.ProviderResourceState{AuditStatus: &status, AuditReply: item.Remote.AuditReply, SyncedAt: &now, ProviderMetadata: item.Remote.ProviderMetadata}
	updates := map[string]any{"audit_status": status, "audit_reply": state.AuditReply, "remote_deleted": false, "synced_at": now, "remote_description": item.Remote.Description}
	if state.ProviderMetadata != nil {
		metadata, err := json.Marshal(state.ProviderMetadata)
		if err != nil {
			return 0, err
		}
		updates["provider_metadata"] = string(metadata)
	}
	if restore {
		updates["deleted_at"] = nil
	}
	if kind == domain.ResourceTemplates {
		t := item.template
		if t != nil {
			var err error
			t, err = dao.LockProviderTemplate(tx, t.ID)
			if err != nil {
				return 0, err
			}
		} else {
			name := item.Remote.Name
			if name == "" {
				name = "供应商模板 " + item.Remote.ID
			}
			t = &model.ProviderTemplate{ProviderID: accountID, TemplateCode: item.Remote.ID, TemplateName: truncateResourceName(name, 200), Status: 1, ContentVersion: 1}
		}
		if item.Parsed == nil {
			return 0, fmt.Errorf("模板原文尚未解析")
		}
		if err := dao.SetTemplateContent(tx, t, item.Remote.Content); err != nil {
			return 0, err
		}
		t.ProviderResourceState = state
		t.ContentType = "text"
		t.Variables = "[]"
		var err error
		t.RemoteName = item.Remote.Name
		t.Category = item.Remote.Category
		t.RemoteDescription = item.Remote.Description
		if restore {
			t.DeletedAt = gorm.DeletedAt{}
		}
		if t.ID == 0 {
			err = tx.Omit(clause.Associations).Create(t).Error
		} else {
			updates["template_content"] = t.TemplateContent
			updates["content_type"] = t.ContentType
			updates["content_version"] = t.ContentVersion
			updates["variables"] = t.Variables
			updates["remote_name"] = t.RemoteName
			updates["category"] = t.Category
			err = tx.Unscoped().Model(&model.ProviderTemplate{}).Where("id = ? AND provider_id = ?", t.ID, accountID).Updates(updates).Error
		}
		return t.ID, err
	}
	t := item.signature
	if t == nil {
		t = &model.ProviderSignature{ProviderAccountID: accountID, SignatureName: truncateResourceName(item.Remote.Content, 100), Status: 1}
	}
	t.ProviderResourceState = state
	t.RemoteID = item.Remote.ID
	updates["remote_id"] = t.RemoteID
	t.SignatureCode = item.Remote.Content
	t.RemoteDescription = item.Remote.Description
	if restore {
		t.DeletedAt = gorm.DeletedAt{}
	}
	var err error
	if t.ID == 0 {
		err = tx.Omit(clause.Associations).Create(t).Error
	} else {
		updates["signature_code"] = t.SignatureCode
		err = tx.Unscoped().Model(&model.ProviderSignature{}).Where("id = ? AND provider_account_id = ?", t.ID, accountID).Updates(updates).Error
	}
	return t.ID, err
}

func truncateResourceName(name string, n int) string {
	r := []rune(name)
	if len(r) > n {
		return string(r[:n])
	}
	return name
}

func (s *AdminProviderResourceService) Mutate(ctx context.Context, accountID uint, kind domain.ResourceKind, action domain.ResourceAction, req ResourceMutationRequest) (*ResourceMutationResult, error) {
	if action != domain.ResourceCreate && action != domain.ResourceUpdate && action != domain.ResourceDelete {
		return nil, domain.ErrResourceUnsupported
	}
	unlock, err := s.acquire(ctx, accountID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	account, definition, err := s.resolve(ctx, accountID, kind)
	if err != nil {
		return nil, err
	}
	if op := definition.Operations[action]; op == nil || op.Handler == nil {
		return nil, domain.ErrResourceUnsupported
	}
	var before *ResourcePreviewItem
	if action != domain.ResourceCreate {
		var rows []domain.RemoteResource
		if definition.Capability().Query {
			rows, err = definition.Execute(ctx, account, domain.ResourceQuery, domain.ResourceInput{ID: req.ID})
		} else {
			rows, err = s.localResourceFacts(ctx, accountID, kind, req.ID)
		}
		if err != nil {
			return nil, err
		}
		if len(rows) != 1 {
			return nil, ErrResourceConflict
		}
		before, err = s.previewItem(s.db.WithContext(ctx), accountID, kind, definition, rows[0])
		if err != nil {
			return nil, err
		}
		if before.Version != req.Version || req.Version == "" {
			return nil, ErrResourceConflict
		}
		if before.IdentityConflict {
			return nil, fmt.Errorf("%s", before.Error)
		}
		if reason := before.OperationErrors[action]; reason != "" {
			return nil, fmt.Errorf("%s", reason)
		}
		if len(before.Impacts) > 0 && !req.ConfirmImpact {
			return nil, fmt.Errorf("请确认受影响的通道后再提交")
		}
	}
	input := req.ResourceInput
	var parsed *domain.ParsedTemplate
	if action != domain.ResourceDelete {
		if utf8.RuneCountInString(input.Name) > 200 {
			return nil, fmt.Errorf("名称不得超过 200 字符")
		}
		if kind == domain.ResourceTemplates {
			meta, lookupErr := s.lookup(account.ProviderCode)
			if lookupErr != nil {
				return nil, lookupErr
			}
			parsed, err = meta.TemplateCodec.Parse(input.Content, account)
			if err != nil {
				return nil, err
			}
		} else if utf8.RuneCountInString(input.Content) > 200 {
			return nil, fmt.Errorf("签名不得超过 200 字符")
		}
	}
	if err := definition.ValidateInput(action, input); err != nil {
		return nil, err
	}
	// Persist the stop before the upstream request. An uncertain write or failed
	// mirror save must not leave the previous configuration eligible for delivery.
	op := definition.Operations[action]
	if kind == domain.ResourceTemplates && before != nil && before.LocalID != 0 &&
		(op.SuspendBeforeWrite || action == domain.ResourceDelete || (action == domain.ResourceUpdate && input.Content != before.template.TemplateContent)) {
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			t, err := dao.LockProviderTemplate(tx, before.LocalID)
			if err != nil {
				return err
			}
			if action == domain.ResourceDelete || input.Content != t.TemplateContent {
				if err := dao.ResetTemplateBindings(tx, t.ID); err != nil {
					return err
				}
			}
			return tx.Unscoped().Model(t).Updates(resourceSuspensionUpdates(before, definition)).Error
		})
		if err != nil {
			return nil, err
		}
		for _, impact := range before.Impacts {
			s.invalidate(impact.ChannelID)
		}
	}
	if (account.ProviderCode == "tencent_sms" || op.SuspendBeforeWrite) && kind == domain.ResourceSignatures && before != nil && before.LocalID != 0 {
		if err := s.db.WithContext(ctx).Unscoped().Model(&model.ProviderSignature{}).Where("id = ? AND provider_account_id = ?", before.LocalID, accountID).Updates(resourceSuspensionUpdates(before, definition)).Error; err != nil {
			return nil, err
		}
		for _, impact := range before.Impacts {
			s.invalidate(impact.ChannelID)
		}
	}
	// Writes are executed once. A transport ambiguity is surfaced to the UI.
	response, err := definition.Execute(ctx, account, action, input)
	if err != nil {
		var remote *domain.RemoteResourceError
		if before != nil && before.LocalID != 0 && definition.AuditOrderID != nil && (!errors.As(err, &remote) || !remote.Uncertain) {
			// A definite rejection did not start a new review. Keep the mirror
			// suspended, but permit a subsequent query/import to restore it.
			if saveErr := s.saveResourceReviewMetadata(ctx, accountID, kind, before.LocalID, localResourceMetadata(before)); saveErr != nil {
				return nil, fmt.Errorf("供应商操作未完成且本地审核标记恢复失败，请核对资源：%w", err)
			}
		}
		return nil, err
	}
	if len(response) != 1 {
		return nil, &domain.RemoteResourceError{Code: "INVALID_RESPONSE", Message: "供应商未返回唯一资源 ID，请查询确认", Uncertain: true}
	}
	id := response[0].ID
	result := &ResourceMutationResult{RemoteID: id, RequestID: response[0].RequestID}
	var item *ResourcePreviewItem
	if action == domain.ResourceDelete {
		item = before
	} else {
		remote := domain.RemoteResource{ResourceInput: input.PublicCopy(definition.Operations[action].Fields)}
		remote.ID = id
		if definition.AuditOrderID != nil {
			remote.ProviderMetadata = cloneResourceMetadata(response[0].ProviderMetadata)
			remote.ProviderMetadata[pendingResourceReview] = map[string]any{"order_id": definition.AuditOrderID(response[0])}
			if before != nil && before.LocalID != 0 {
				if err := s.saveResourceReviewMetadata(ctx, accountID, kind, before.LocalID, remote.ProviderMetadata); err != nil {
					result.PendingSync = true
					result.Warning = "供应商已受理，本地工单保存失败，请重新查询并同步"
					return result, nil
				}
			}
		}
		if before != nil && remote.Category == "" {
			remote.Category = before.Remote.Category
		}
		// A successful mutation acknowledgement does not constitute audit approval.
		queried, queryErr := definition.Execute(ctx, account, domain.ResourceQuery, domain.ResourceInput{ID: id})
		if queryErr == nil && len(queried) == 1 {
			fact := queried[0]
			matchesOrder := definition.AuditOrderID == nil || (definition.AuditOrderID(response[0]) != "" && definition.AuditOrderID(response[0]) == definition.AuditOrderID(fact))
			if fact.Content == input.Content && matchesOrder {
				remote.AuditStatus = fact.AuditStatus
				remote.AuditReply = fact.AuditReply
				remote.ProviderMetadata = fact.ProviderMetadata
			} else {
				result.Warning = "供应商查询尚未反映本次修改，请稍后刷新审核状态"
			}
		} else {
			result.Warning = "供应商已受理，审核状态待查询确认"
		}
		item, err = s.previewItem(s.db.WithContext(ctx), accountID, kind, definition, remote)
		if err != nil {
			result.PendingSync = true
			result.Warning = "供应商操作成功，本地记录待同步，请从供应商查询导入"
			return result, nil
		}
		if item.Error != "" {
			result.PendingSync = true
			result.Warning = "供应商操作成功，本地匹配冲突：" + item.Error
			return result, nil
		}
		if parsed != nil {
			item.Parsed = parsed
		}
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if action == domain.ResourceDelete {
			result.LocalID = item.LocalID
			if item.LocalID == 0 {
				return nil
			}
			updates := map[string]any{"remote_deleted": true, "audit_status": int8(0), "synced_at": time.Now().UTC()}
			if kind == domain.ResourceTemplates {
				return tx.Unscoped().Model(&model.ProviderTemplate{}).Where("id = ? AND provider_id = ?", item.LocalID, accountID).Updates(updates).Error
			}
			return tx.Unscoped().Model(&model.ProviderSignature{}).Where("id = ? AND provider_account_id = ?", item.LocalID, accountID).Updates(updates).Error
		}
		var saveErr error
		result.LocalID, saveErr = s.saveMirror(tx, accountID, kind, item, false)
		return saveErr
	})
	if err != nil {
		result.PendingSync = true
		result.Warning = "供应商操作成功，本地保存失败，请重新查询并同步"
		return result, nil
	}
	for _, impact := range item.Impacts {
		s.invalidate(impact.ChannelID)
	}
	return result, nil
}
