package service

import (
	"context"
	"encoding/json"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

// The marker survives a process crash between an upstream write and its local mirror.
// It is only used by providers that expose a review/order identity.
const pendingResourceReview = "pending_review"

func cloneResourceMetadata(metadata map[string]any) map[string]any {
	result := map[string]any{}
	if encoded, err := json.Marshal(metadata); err == nil {
		_ = json.Unmarshal(encoded, &result)
	}
	if result == nil {
		result = map[string]any{}
	}
	return result
}

func localResourceMetadata(item *ResourcePreviewItem) map[string]any {
	if item.template != nil {
		return item.template.ProviderMetadata
	}
	if item.signature != nil {
		return item.signature.ProviderMetadata
	}
	return nil
}

func resourceSuspensionUpdates(before *ResourcePreviewItem, definition *domain.ResourceDefinition) map[string]any {
	updates := map[string]any{"audit_status": 0, "synced_at": time.Now().UTC()}
	if definition.AuditOrderID != nil {
		metadata := cloneResourceMetadata(localResourceMetadata(before))
		metadata[pendingResourceReview] = map[string]any{"previous_order_id": definition.AuditOrderID(before.Remote)}
		encoded, _ := json.Marshal(metadata)
		updates["provider_metadata"] = string(encoded)
	}
	return updates
}

func (s *AdminProviderResourceService) saveResourceReviewMetadata(ctx context.Context, accountID uint, kind domain.ResourceKind, localID uint, metadata map[string]any) error {
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	db := s.db.WithContext(ctx).Unscoped()
	if kind == domain.ResourceTemplates {
		db = db.Model(&model.ProviderTemplate{}).Where("id = ? AND provider_id = ?", localID, accountID)
	} else {
		db = db.Model(&model.ProviderSignature{}).Where("id = ? AND provider_account_id = ?", localID, accountID)
	}
	return db.Updates(map[string]any{"provider_metadata": string(encoded)}).Error
}

func reconcileResourceReview(item *ResourcePreviewItem, definition *domain.ResourceDefinition) {
	if definition.AuditOrderID == nil {
		return
	}
	stored := localResourceMetadata(item)
	orderID := definition.AuditOrderID(item.Remote)
	if pending, ok := stored[pendingResourceReview].(map[string]any); ok {
		wanted, _ := pending["order_id"].(string)
		previous, _ := pending["previous_order_id"].(string)
		matched := orderID != "" && (wanted != "" && orderID == wanted || wanted == "" && previous != "" && orderID != previous)
		if !matched {
			item.Remote.AuditStatus = 0
			item.Error = "供应商尚未返回本次操作的审核工单，请稍后查询同步"
			item.OperationErrors[domain.ResourceUpdate] = item.Error
			item.OperationErrors[domain.ResourceDelete] = item.Error
			return
		}
	}
	// List responses omit some detail fields. Preserve known metadata only when
	// both reads identify the same review, never carry facts across review orders.
	if orderID != "" && orderID == definition.AuditOrderID(domain.RemoteResource{ProviderMetadata: stored}) {
		merged := cloneResourceMetadata(stored)
		delete(merged, pendingResourceReview)
		for key, value := range item.Remote.ProviderMetadata {
			oldMap, oldOK := merged[key].(map[string]any)
			newMap, newOK := value.(map[string]any)
			if oldOK && newOK {
				for k, v := range newMap {
					oldMap[k] = v
				}
			} else {
				merged[key] = value
			}
		}
		item.Remote.ProviderMetadata = merged
	}
}
