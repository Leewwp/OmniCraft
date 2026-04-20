package model

import "time"

type IP struct {
	ID          int64      `gorm:"primaryKey;autoIncrement"`
	Name        string     `gorm:"size:255;not null"`
	Slug        string     `gorm:"size:255;uniqueIndex;not null"`
	Description string     `gorm:"type:text"`
	CoverURL    string     `gorm:"type:text"`
	Category    string     `gorm:"size:50"`
	CreatorID   *int64     `gorm:"index"`
	Creator     *User      `gorm:"foreignKey:CreatorID"`
	Status      string     `gorm:"size:20;not null;default:pending"`
	CreatedAt   time.Time  `gorm:"autoCreateTime"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime"`
}

func (IP) TableName() string { return "ips" }

type IPReviewLog struct {
	ID         int64      `gorm:"primaryKey;autoIncrement"`
	IPID       int64      `gorm:"not null;index:idx_ip_review_logs_ip"`
	IP         IP         `gorm:"foreignKey:IPID"`
	ReviewerID *int64
	Reviewer   *User  `gorm:"foreignKey:ReviewerID"`
	Action     string `gorm:"size:20;not null"`
	Reason     string `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

func (IPReviewLog) TableName() string { return "ip_review_logs" }

type IPTag struct {
	IPID int64  `gorm:"primaryKey"`
	Tag  string `gorm:"size:50;primaryKey"`
}

func (IPTag) TableName() string { return "ip_tags" }
