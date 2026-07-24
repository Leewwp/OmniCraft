package model

import "time"

// NotificationBroadcastRequest is the durable idempotency record for an admin
// broadcast. It stores only SHA-256 hashes of the idempotency key and of the
// normalized payload — never the raw key, title, or Markdown body.
type NotificationBroadcastRequest struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ActorID        int64     `gorm:"not null;uniqueIndex:uq_notification_broadcast_requests_actor_key,priority:1" json:"actor_id"`
	KeyHash        string    `gorm:"size:64;not null;uniqueIndex:uq_notification_broadcast_requests_actor_key,priority:2" json:"-"`
	PayloadHash    string    `gorm:"size:64;not null" json:"-"`
	RecipientCount int       `gorm:"not null" json:"recipient_count"`
	BroadcastAt    time.Time `gorm:"not null" json:"broadcast_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (NotificationBroadcastRequest) TableName() string {
	return "notification_broadcast_requests"
}
