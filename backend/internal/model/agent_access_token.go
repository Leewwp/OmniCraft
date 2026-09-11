package model

import "time"

// AgentAccessToken is the PAT machine identity row (SP-16 #450, spec D2).
// The plaintext token is shown once at creation; only its SHA-256 hex lives
// here. No expiry by design (2026-09-11 user decision) — revocation is the
// stop-leak lever and last_used_at lets the owner spot anomalies.
type AgentAccessToken struct {
	ID          int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int64      `gorm:"not null;index" json:"user_id"`
	Name        string     `gorm:"size:64;not null" json:"name"`
	TokenHash   string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	TokenPrefix string     `gorm:"size:12;not null" json:"token_prefix"`
	Scopes      string     `gorm:"size:32;not null;default:download" json:"scopes"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (AgentAccessToken) TableName() string {
	return "agent_access_tokens"
}
