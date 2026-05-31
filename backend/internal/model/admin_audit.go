package model

import (
	"time"
)

type AdminAuditLog struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AdminUserID  int64     `gorm:"not null;index" json:"admin_user_id"`
	Action       string    `gorm:"not null;size:96;index" json:"action"`
	TargetType   string    `gorm:"not null;size:48" json:"target_type"`
	TargetID     string    `gorm:"size:96" json:"target_id,omitempty"`
	TraceID      string    `gorm:"size:96" json:"trace_id,omitempty"`
	Metadata     JSONMap   `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	Result       string    `gorm:"not null;size:24" json:"result"`
	CreatedAt    time.Time `gorm:"index:idx_admin_audit_logs_created_at" json:"created_at"`
}

func (AdminAuditLog) TableName() string {
	return "admin_audit_logs"
}
