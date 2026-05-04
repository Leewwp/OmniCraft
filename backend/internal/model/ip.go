package model

import "time"

type IP struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"size:255;not null" json:"name"`
	Slug        string    `gorm:"size:255;uniqueIndex;not null" json:"slug"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	CoverURL    string    `gorm:"type:text" json:"cover_url,omitempty"`
	Category    string    `gorm:"size:50" json:"category,omitempty"`
	CreatorID   *int64    `gorm:"index" json:"creator_id,omitempty"`
	Creator     *User     `gorm:"foreignKey:CreatorID" json:"creator,omitempty"`
	Status      string    `gorm:"size:20;not null;default:pending" json:"status"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (IP) TableName() string { return "ips" }

type IPReviewLog struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	IPID       int64     `gorm:"not null;index:idx_ip_review_logs_ip" json:"ip_id"`
	IP         IP        `gorm:"foreignKey:IPID" json:"-"`
	ReviewerID *int64    `json:"reviewer_id,omitempty"`
	Reviewer   *User     `gorm:"foreignKey:ReviewerID" json:"-"`
	Action     string    `gorm:"size:20;not null" json:"action"`
	Reason     string    `gorm:"type:text" json:"reason,omitempty"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (IPReviewLog) TableName() string { return "ip_review_logs" }

type IPTag struct {
	IPID int64  `gorm:"primaryKey" json:"ip_id"`
	Tag  string `gorm:"size:50;primaryKey" json:"tag"`
}

func (IPTag) TableName() string { return "ip_tags" }
