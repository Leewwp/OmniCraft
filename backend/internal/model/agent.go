package model

import "time"

type AgentConversation struct {
	ID int64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// 089 起可空（guest 行 user_id 为 NULL，归属见 is_guest+guest_device_key
	// 与 agent_conversations_owner_check）；DDL 真源是迁移文件，此标签仅供
	// 测试 AutoMigrate 对齐形状。
	UserID      int64      `gorm:"index" json:"user_id"`
	ContextType string     `gorm:"size:50;not null;default:''" json:"context_type"`
	ContextID   *int64     `json:"context_id,omitempty"`
	Title       *string    `gorm:"size:200" json:"title,omitempty"`
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type AgentMessage struct {
	ID             int64   `gorm:"primaryKey;autoIncrement" json:"id"`
	ConversationID int64   `gorm:"not null;index" json:"conversation_id"`
	Role           string  `gorm:"size:20;not null" json:"role"`
	Content        *string `json:"content,omitempty"`
	ToolCalls      JSONMap `gorm:"type:jsonb" json:"tool_calls,omitempty"`
	// N4: the done event's validated citations persist on the answer row so
	// the history endpoint can replay jump entries for past turns. Nullable
	// jsonb — legacy rows (and think rows) keep NULL = "no citations".
	Citations []AgentCitation `gorm:"serializer:json;type:jsonb;column:citations" json:"citations,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// AgentCitation is the server-owned citation contract shared by the stream
// done event and the persisted/history projections (moved to the model layer
// for N4 persistence; the service aliases it as its canonical name).
type AgentCitation struct {
	ContentID      int64  `json:"content_id"`
	ContentVersion int    `json:"content_version"`
	ChunkKey       string `json:"chunk_key"`
	ChunkIndex     int    `json:"chunk_index"`
	Title          string `json:"title"`
	Zone           string `json:"zone"`
	Route          string `json:"route"`
	Excerpt        string `json:"excerpt"`
	Source         string `json:"source"`
	// Category carries the IP category slug for zone="ip" citations
	// (SP-19 G2-1); content citations leave it empty.
	Category string `json:"category,omitempty"`
	// Number is the turn-global citation number assigned when the candidate
	// enters the pool (FT-5 #697): it accumulates across every search call
	// of the turn and revalidation drops keep their slot, so inline [n]
	// markers stay aligned with the emitted card. 0 = legacy/positional
	// (rows persisted before FT-5, citations built outside the turn pool).
	Number int `json:"number,omitempty"`
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
