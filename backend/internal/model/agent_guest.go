package model

import "time"

// AgentGuestConversation is the guest-shaped projection of the shared
// agent_conversations table (#854). Guest rows are owned by a signed device
// key with user_id NULL — never a sentinel user id, never a pseudo user row.
// The 089 CHECK constraint (agent_conversations_owner_check) makes the two
// ownership shapes mutually exclusive at the database level; writes to guest
// rows go through this struct so the column set cannot drift into a mixed
// shape. Reads of the shared columns reuse model.AgentConversation.
type AgentGuestConversation struct {
	ID             int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	IsGuest        bool   `gorm:"column:is_guest;not null;default:true" json:"is_guest"`
	GuestDeviceKey string `gorm:"column:guest_device_key;size:64;not null" json:"guest_device_key"`
	ContextType    string `gorm:"size:50;not null;default:''" json:"context_type"`
	ContextID      *int64 `json:"context_id,omitempty"`
	// Guests cannot rename or pin: the columns exist for the shared table,
	// the guest surface never writes them.
	Title     *string    `gorm:"size:200" json:"title,omitempty"`
	PinnedAt  *time.Time `json:"pinned_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TableName pins the shared table (GORM would otherwise pluralize the struct
// name into agent_guest_conversations).
func (AgentGuestConversation) TableName() string { return "agent_conversations" }
