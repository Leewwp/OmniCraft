package model

import "time"

type Notification struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     int64     `gorm:"not null;index" json:"user_id"`
	Type       string    `gorm:"size:50;not null" json:"type"`
	Channel    string    `gorm:"size:20;not null" json:"channel"`
	Title      *string   `gorm:"size:500" json:"title,omitempty"`
	Body       *string   `json:"body,omitempty"`
	TargetType *string   `gorm:"size:50" json:"target_type,omitempty"`
	TargetID   *int64    `json:"target_id,omitempty"`
	SenderID   *int64    `json:"sender_id,omitempty"`
	IsRead     bool      `gorm:"not null;default:false" json:"is_read"`
	CreatedAt  time.Time `json:"created_at"`
}

type Conversation struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConversationParticipant struct {
	ConversationID int64      `gorm:"primaryKey" json:"conversation_id"`
	UserID         int64      `gorm:"primaryKey" json:"user_id"`
	LastReadAt     *time.Time `json:"last_read_at,omitempty"`
	UnreadCount    int        `gorm:"not null;default:0" json:"unread_count"`
	LeftAt         *time.Time `json:"left_at,omitempty"`
}

type Message struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID int64     `gorm:"not null;index" json:"conversation_id"`
	SenderID       int64     `gorm:"not null" json:"sender_id"`
	Body           string    `gorm:"not null" json:"body"`
	MsgType        string    `gorm:"not null;default:'text';size:20" json:"msg_type"`
	Metadata       JSONMap   `gorm:"type:jsonb;not null;default:'{}'" json:"metadata,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type RehabCourse struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ViolationType string    `gorm:"size:100;not null;uniqueIndex" json:"violation_type"`
	ContentI18n   JSONMap   `gorm:"type:jsonb;not null;default:'{}'" json:"content_i18n"`
	MinReadingSec int       `gorm:"not null;default:60" json:"min_reading_sec"`
	RewardPoints  int       `gorm:"not null;default:0" json:"reward_points"`
	CreatedAt     time.Time `json:"created_at"`
}

type RehabCompletion struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64      `gorm:"not null;index;uniqueIndex:idx_rehab_unique" json:"user_id"`
	CourseID    int64      `gorm:"not null;uniqueIndex:idx_rehab_unique" json:"course_id"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
