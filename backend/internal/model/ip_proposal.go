package model

import (
	"time"
)

// IPProposal is a field-level collaborative-governance proposal on an IP's
// profile (#290): description / cover / tag changes that the IP's followers
// vote on. Independent from the judge domain — proposals edit IP data, judge
// cases arbitrate content violations; the two must never share tables or
// config.
type IPProposal struct {
	ID                int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	IPID              int64      `gorm:"not null;index:idx_ip_proposals_ip_status,priority:1" json:"ip_id"`
	IP                *IP        `gorm:"foreignKey:IPID" json:"-"`
	ProposerID        int64      `gorm:"not null" json:"proposer_id"`
	Proposer          *User      `gorm:"foreignKey:ProposerID" json:"proposer,omitempty"`
	Status            string     `gorm:"size:16;not null;default:open" json:"status"`
	DescriptionChange *string    `gorm:"type:text" json:"description_change,omitempty"`
	CoverURLChange    *string    `gorm:"type:text" json:"cover_url_change,omitempty"`
	TagsAdd           string     `gorm:"type:jsonb;not null;default:[]" json:"-"`
	TagsRemove        string     `gorm:"type:jsonb;not null;default:[]" json:"-"`
	ModerationState   string     `gorm:"size:16;not null;default:approved" json:"moderation_state"`
	YesVotes          int        `gorm:"not null;default:0" json:"yes_votes"`
	NoVotes           int        `gorm:"not null;default:0" json:"no_votes"`
	CreatedAt         time.Time  `gorm:"autoCreateTime" json:"created_at"`
	DeadlineAt        time.Time  `gorm:"not null" json:"deadline_at"`
	ClosedAt          *time.Time `json:"closed_at,omitempty"`
	EffectiveAt       *time.Time `json:"effective_at,omitempty"`
}

func (IPProposal) TableName() string { return "ip_proposals" }

type IPProposalVote struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ProposalID int64     `gorm:"not null;uniqueIndex:uq_ip_proposal_votes_one_per_voter,priority:1" json:"proposal_id"`
	VoterID    int64     `gorm:"not null;uniqueIndex:uq_ip_proposal_votes_one_per_voter,priority:2" json:"voter_id"`
	Vote       string    `gorm:"size:8;not null" json:"vote"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (IPProposalVote) TableName() string { return "ip_proposal_votes" }

// IPProfileVersion snapshots the pre-change IP profile for every adopted
// proposal; the append-only timeline is the provenance of the IP data.
type IPProfileVersion struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	IPID       int64     `gorm:"not null;index:idx_ip_profile_versions_ip,priority:1" json:"ip_id"`
	ProposalID int64     `gorm:"not null" json:"proposal_id"`
	Snapshot   string    `gorm:"type:jsonb;not null" json:"-"`
	Changes    string    `gorm:"type:jsonb;not null" json:"-"`
	YesVotes   int       `gorm:"not null;default:0" json:"yes_votes"`
	NoVotes    int       `gorm:"not null;default:0" json:"no_votes"`
	CreatedAt  time.Time `gorm:"autoCreateTime;index:idx_ip_profile_versions_ip,priority:2" json:"created_at"`
}

func (IPProfileVersion) TableName() string { return "ip_profile_versions" }
