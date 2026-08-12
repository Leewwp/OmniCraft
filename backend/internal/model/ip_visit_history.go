package model

import "time"

// IPVisitHistory records the most recent visit of one account to one IP
// entity. One row exists per (user_id, ip_id) pair; repeated visits update
// visited_at without creating new rows. It is independent of content browse
// history so signed-in recency never mixes with reading history.
type IPVisitHistory struct {
	UserID    int64     `gorm:"primaryKey" json:"user_id"`
	IPID      int64     `gorm:"primaryKey" json:"ip_id"`
	VisitedAt time.Time `gorm:"not null" json:"visited_at"`
}

func (IPVisitHistory) TableName() string { return "ip_visit_history" }
