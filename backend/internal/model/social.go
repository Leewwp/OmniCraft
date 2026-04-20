package model

import "time"

type Discussion struct {
	ID            int64  `gorm:"primaryKey;autoIncrement"`
	IPID          *int64 `gorm:"index"`
	IP            *IP    `gorm:"foreignKey:IPID"`
	ContentItemID *int64
	ContentItem   *ContentItem `gorm:"foreignKey:ContentItemID"`
	AuthorID      int64        `gorm:"not null;index"`
	Author        User         `gorm:"foreignKey:AuthorID"`
	Title         string       `gorm:"size:500;not null"`
	Body          string       `gorm:"type:text"`
	Status        string       `gorm:"size:20;not null;default:published"`
	IsPinned      bool         `gorm:"not null;default:false"`
	ViewCount     int64        `gorm:"not null;default:0"`
	ReplyCount    int          `gorm:"not null;default:0"`
	LastActiveAt  time.Time    `gorm:"not null;default:NOW()"`
	CreatedAt     time.Time    `gorm:"autoCreateTime"`
	UpdatedAt     time.Time    `gorm:"autoUpdateTime"`
}

func (Discussion) TableName() string { return "discussions" }

type Comment struct {
	ID            int64        `gorm:"primaryKey;autoIncrement"`
	ContentItemID *int64       `gorm:"index"`
	ContentItem   *ContentItem `gorm:"foreignKey:ContentItemID"`
	DiscussionID  *int64       `gorm:"index"`
	Discussion    *Discussion  `gorm:"foreignKey:DiscussionID"`
	ParentID      *int64
	Parent        *Comment  `gorm:"foreignKey:ParentID"`
	AuthorID      int64     `gorm:"not null;index"`
	Author        User      `gorm:"foreignKey:AuthorID"`
	TargetType    string    `gorm:"size:20;index"`
	TargetID      int64     `gorm:"index"`
	Content       string    `gorm:"type:text"`
	Body          string    `gorm:"type:text;not null"`
	Status        string    `gorm:"size:20;not null;default:published"`
	LikeCount     int       `gorm:"not null;default:0"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}

func (Comment) TableName() string { return "comments" }

type Reaction struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	UserID     int64     `gorm:"not null;uniqueIndex:idx_reactions_unique"`
	User       User      `gorm:"foreignKey:UserID"`
	TargetType string    `gorm:"size:20;not null;uniqueIndex:idx_reactions_unique"`
	TargetID   int64     `gorm:"not null;uniqueIndex:idx_reactions_unique"`
	Reaction   string    `gorm:"size:10;not null"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

func (Reaction) TableName() string { return "reactions" }

type Favorite struct {
	UserID        int64       `gorm:"primaryKey"`
	ContentItemID int64       `gorm:"primaryKey"`
	User          User        `gorm:"foreignKey:UserID"`
	ContentItem   ContentItem `gorm:"foreignKey:ContentItemID"`
	CreatedAt     time.Time   `gorm:"autoCreateTime"`
}

func (Favorite) TableName() string { return "favorites" }

type BrowseHistory struct {
	ID            int64       `gorm:"primaryKey;autoIncrement"`
	UserID        int64       `gorm:"not null;uniqueIndex:idx_browse_unique;index:idx_browse_history_user"`
	ContentItemID int64       `gorm:"not null;uniqueIndex:idx_browse_unique"`
	User          User        `gorm:"foreignKey:UserID"`
	ContentItem   ContentItem `gorm:"foreignKey:ContentItemID"`
	ViewedAt      time.Time   `gorm:"autoCreateTime"`
}

func (BrowseHistory) TableName() string { return "browse_history" }

type Follow struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	FollowerID int64     `gorm:"not null;uniqueIndex:idx_follows_unique;index"`
	Follower   User      `gorm:"foreignKey:FollowerID"`
	TargetType string    `gorm:"size:20;not null;uniqueIndex:idx_follows_unique"`
	TargetID   int64     `gorm:"not null;uniqueIndex:idx_follows_unique"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}

func (Follow) TableName() string { return "follows" }

type Appeal struct {
	ID             int64  `gorm:"primaryKey;autoIncrement"`
	UserID         int64  `gorm:"not null;index"`
	User           User   `gorm:"foreignKey:UserID"`
	TargetType     string `gorm:"size:20;not null"`
	TargetID       int64  `gorm:"not null"`
	Reason         string `gorm:"type:text;not null"`
	Status         string `gorm:"size:20;not null;default:pending;index"`
	AdminResponse  string `gorm:"type:text"`
	ResolvedBy     *int64
	ResolvedByUser *User `gorm:"foreignKey:ResolvedBy"`
	ResolvedAt     *time.Time
	CreatedAt      time.Time `gorm:"autoCreateTime"`
}

func (Appeal) TableName() string { return "appeals" }
