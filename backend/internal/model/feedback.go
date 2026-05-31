package model

import (
	"time"
)

type FeedbackTicket struct {
	ID              int64         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          *int64        `gorm:"index" json:"user_id,omitempty"`
	ContactEmail    string        `gorm:"size:255" json:"contact_email,omitempty"`
	Category        string        `gorm:"not null;size:32" json:"category"`
	Title           string        `gorm:"not null;size:160" json:"title"`
	Description     string        `gorm:"not null;type:text" json:"description"`
	DiagnosticSummary JSONMap     `gorm:"type:jsonb;not null;default:'{}'" json:"diagnostic_summary"`
	Status          string        `gorm:"not null;size:24;default:'open'" json:"status"`
	Priority        string        `gorm:"not null;size:24;default:'normal'" json:"priority"`
	AssigneeAdminID *int64        `json:"assignee_admin_id,omitempty"`
	Replies         []FeedbackReply `gorm:"foreignKey:TicketID" json:"replies,omitempty"`
	Attachments     []FeedbackAttachment `gorm:"foreignKey:TicketID" json:"attachments,omitempty"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	ResolvedAt      *time.Time    `json:"resolved_at,omitempty"`
}

func (FeedbackTicket) TableName() string {
	return "feedback_tickets"
}

type FeedbackReply struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TicketID      int64     `gorm:"not null;index" json:"ticket_id"`
	AuthorUserID  *int64    `json:"author_user_id,omitempty"`
	AuthorAdminID *int64    `json:"author_admin_id,omitempty"`
	Body          string    `gorm:"not null;type:text" json:"body"`
	IsInternalNote bool     `gorm:"not null;default:false" json:"is_internal_note"`
	CreatedAt     time.Time `json:"created_at"`
}

func (FeedbackReply) TableName() string {
	return "feedback_replies"
}

type FeedbackAttachment struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TicketID  int64     `gorm:"not null;index" json:"ticket_id"`
	OSSKey    string    `gorm:"not null;type:text" json:"oss_key"`
	FileType  string    `gorm:"not null;size:32" json:"file_type"`
	MimeType  string    `gorm:"not null;size:100" json:"mime_type"`
	SizeBytes int64     `gorm:"not null" json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

func (FeedbackAttachment) TableName() string {
	return "feedback_attachments"
}
