package model

import (
	"time"
)

// AgentDraft is one server-side draft in the agent workspace (SP-23 M2):
// the document-editing MCP server reads, analyses and patches these rows
// for the bound user. The publish form's localStorage drafts are a separate
// client-side concept; nothing here feeds the frozen golden set or the RAG
// index.
type AgentDraft struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	UserID      int64     `gorm:"not null;index" json:"user_id"`
	Title       string    `gorm:"size:200;not null;default:''" json:"title"`
	ContentType string    `gorm:"size:20;not null;default:'article'" json:"content_type"`
	Body        string    `gorm:"type:text;not null;default:''" json:"body"`
	Tags        JSONB     `gorm:"type:jsonb;not null;default:'[]'" json:"tags"`
	Status      string    `gorm:"size:16;not null;default:'active'" json:"status"`
}

func (AgentDraft) TableName() string { return "agent_drafts" }
