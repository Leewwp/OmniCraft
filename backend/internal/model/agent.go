package model

import "time"

type AgentConversation struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64     `gorm:"not null;index" json:"user_id"`
	ContextType string    `gorm:"size:50;not null;default:''" json:"context_type"`
	ContextID   *int64    `json:"context_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AgentMessage struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID int64     `gorm:"not null;index" json:"conversation_id"`
	Role           string    `gorm:"size:20;not null" json:"role"`
	Content        *string   `json:"content,omitempty"`
	ToolCalls      JSONMap   `gorm:"type:jsonb" json:"tool_calls,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
