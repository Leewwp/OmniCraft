package model

import "time"

const (
	CollabInviteStatusPending  = "pending"
	CollabInviteStatusAccepted = "accepted"
	CollabInviteStatusDeclined = "declined"
	CollabInviteStatusExpired  = "expired"
)

type CollabInvite struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	ContentID   int64      `gorm:"not null;index" json:"content_id"`
	InviterID   int64      `gorm:"not null;index" json:"inviter_id"`
	InviteeID   int64      `gorm:"not null;index" json:"invitee_id"`
	MessageID   *int64     `json:"message_id,omitempty"`
	Status      string     `gorm:"not null;default:'pending';size:20" json:"status"`
	ExpiresAt   time.Time  `gorm:"not null" json:"expires_at"`
	RespondedAt *time.Time `json:"responded_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (CollabInvite) TableName() string {
	return "collaboration_invites"
}
