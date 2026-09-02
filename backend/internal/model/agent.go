package model

import "time"

type AgentConversation struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64      `gorm:"not null;index" json:"user_id"`
	ContextType string     `gorm:"size:50;not null;default:''" json:"context_type"`
	ContextID   *int64     `json:"context_id,omitempty"`
	Title       *string    `gorm:"size:200" json:"title,omitempty"`
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type AgentMessage struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID int64     `gorm:"not null;index" json:"conversation_id"`
	Role           string    `gorm:"size:20;not null" json:"role"`
	Content        *string   `json:"content,omitempty"`
	ToolCalls      JSONMap   `gorm:"type:jsonb" json:"tool_calls,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// AgentChatSurface is a server-owned enum describing where a chat request was
// initiated. Client-provided summaries (title/type/route) are never trusted;
// the service reloads resource context from the database using the current
// viewer, and the surface only selects server-owned prompt text.
type AgentChatSurface string

const (
	AgentChatSurfaceGlobal  AgentChatSurface = "global"
	AgentChatSurfaceContent AgentChatSurface = "content"
	AgentChatSurfaceSearch  AgentChatSurface = "search"
	AgentChatSurfacePublish AgentChatSurface = "publish"
)

// AgentChatContext is the only client-authored chat context accepted by the
// web agent. It carries a server-owned surface enum and an optional content ID
// that the service re-validates with the current viewer's visibility scope.
type AgentChatContext struct {
	Surface   AgentChatSurface `json:"surface"`
	ContentID *int64           `json:"content_id,omitempty"`
}
