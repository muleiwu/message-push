package model

import "time"

// ProviderResourceState is independent of the local administrator's status switch.
// NULL audit status denotes an existing manually maintained resource.
type ProviderResourceState struct {
	RemoteID      string     `gorm:"type:varchar(100);default:''" json:"remote_id"`
	AuditStatus   *int8      `json:"audit_status"`
	AuditReply    string     `gorm:"type:text" json:"audit_reply"`
	RemoteDeleted bool       `gorm:"not null;default:false" json:"remote_deleted"`
	SyncedAt      *time.Time `json:"synced_at"`
}

func (s ProviderResourceState) RemoteUsable() bool {
	return !s.RemoteDeleted && (s.AuditStatus == nil || *s.AuditStatus == 2)
}
func (t *ProviderTemplate) Usable() bool  { return t != nil && t.Status == 1 && t.RemoteUsable() }
func (s *ProviderSignature) Usable() bool { return s != nil && s.Status == 1 && s.RemoteUsable() }

// ResourceUsableSQL is shared by candidate and delivery lookups.
// table is an internal table name, never request input.
func ResourceUsableSQL(table string) string {
	return "(" + table + ".audit_status IS NULL OR " + table + ".audit_status = 2) AND " + table + ".remote_deleted = false"
}
