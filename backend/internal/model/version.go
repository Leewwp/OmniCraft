package model

import "time"

type ContentVersion struct {
	ID              int64          `gorm:"primaryKey;autoIncrement"`
	ContentItemID   int64          `gorm:"not null;index"`
	ContentItem     ContentItem    `gorm:"foreignKey:ContentItemID"`
	ParentVersionID *int64
	ParentVersion   *ContentVersion `gorm:"foreignKey:ParentVersionID"`
	AuthorID        int64           `gorm:"not null"`
	Author          User            `gorm:"foreignKey:AuthorID"`
	VersionNumber   int             `gorm:"not null"`
	StorageType     string          `gorm:"size:10;not null"`
	StorageKey      string          `gorm:"type:text"`
	DiffSummary     string          `gorm:"type:text"`
	Status          string          `gorm:"size:20;not null;default:active"`
	IsLatest        bool            `gorm:"not null;default:false"`
	CreatedAt       time.Time       `gorm:"autoCreateTime"`
}

func (ContentVersion) TableName() string { return "content_versions" }

type PullRequest struct {
	ID                int64          `gorm:"primaryKey;autoIncrement"`
	ContentItemID     int64          `gorm:"not null;index:idx_pr_content"`
	ContentItem       ContentItem    `gorm:"foreignKey:ContentItemID"`
	SubmitterID       int64          `gorm:"not null;index"`
	Submitter         User           `gorm:"foreignKey:SubmitterID"`
	BaseVersionID     int64          `gorm:"not null"`
	BaseVersion       ContentVersion `gorm:"foreignKey:BaseVersionID"`
	ProposedVersionID *int64
	ProposedVersion   *ContentVersion `gorm:"foreignKey:ProposedVersionID"`
	Status            string          `gorm:"size:20;not null;default:open"`
	Message           string          `gorm:"type:text"`
	RejectReason      string          `gorm:"type:text"`
	ResolvedAt        *time.Time
	CreatedAt         time.Time `gorm:"autoCreateTime"`
}

func (PullRequest) TableName() string { return "pull_requests" }

type ContentContributor struct {
	ContentItemID int64       `gorm:"primaryKey"`
	UserID        int64       `gorm:"primaryKey"`
	ContentItem   ContentItem `gorm:"foreignKey:ContentItemID"`
	User          User        `gorm:"foreignKey:UserID"`
	PRCount       int         `gorm:"not null;default:1"`
	FirstAt       time.Time   `gorm:"autoCreateTime"`
}

func (ContentContributor) TableName() string { return "content_contributors" }

type AuthorBlocklist struct {
	AuthorID  int64     `gorm:"primaryKey"`
	BlockedID int64     `gorm:"primaryKey"`
	Author    User      `gorm:"foreignKey:AuthorID"`
	Blocked   User      `gorm:"foreignKey:BlockedID"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

func (AuthorBlocklist) TableName() string { return "author_blocklist" }
