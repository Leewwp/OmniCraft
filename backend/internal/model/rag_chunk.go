package model

import (
	"time"

	"github.com/lib/pq"
)

type RagChunk struct {
	ID              int64          `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	ContentID       int64          `gorm:"not null" json:"content_id"`
	ContentVersion  int            `gorm:"not null" json:"content_version"`
	ChunkIndex      int            `gorm:"not null" json:"chunk_index"`
	ChunkKey        string         `gorm:"type:char(64);not null" json:"chunk_key"`
	ChunkingVersion int            `gorm:"not null" json:"chunking_version"`
	Heading         string         `gorm:"type:text;not null" json:"heading"`
	Text            string         `gorm:"type:text;not null" json:"text"`
	SourceStart     int            `gorm:"not null" json:"source_start"`
	SourceEnd       int            `gorm:"not null" json:"source_end"`
	Zone            string         `gorm:"size:10;not null" json:"zone"`
	ContentType     string         `gorm:"size:20;not null" json:"content_type"`
	Category        *string        `gorm:"size:50" json:"category,omitempty"`
	IP              *int64         `json:"ip,omitempty"`
	Tags            pq.StringArray `gorm:"type:text[];not null;default:'{}'" json:"tags"`
	IndexVersion    int            `gorm:"not null" json:"index_version"`
}

func (RagChunk) TableName() string { return "rag_chunks" }

type ChunkEmbedding struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	ChunkID        int64     `gorm:"not null" json:"chunk_id"`
	Embedding      string    `gorm:"type:vector(1536);not null" json:"-"`
	EmbeddingModel string    `gorm:"size:100;not null" json:"embedding_model"`
	EmbeddedAt     time.Time `json:"embedded_at"`
}

func (ChunkEmbedding) TableName() string { return "chunk_embeddings" }

type IndexProjectionStatus struct {
	ID              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt       time.Time  `json:"created_at"`
	ContentID       int64      `gorm:"not null" json:"content_id"`
	IndexVersion    int        `gorm:"not null" json:"index_version"`
	ChunkingVersion int        `gorm:"not null" json:"chunking_version"`
	EmbeddingModel  string     `gorm:"size:100;not null" json:"embedding_model"`
	State           string     `gorm:"size:20;not null" json:"state"`
	ErrorSummary    string     `gorm:"type:text;not null" json:"error_summary"`
	LastIndexedAt   *time.Time `json:"last_indexed_at,omitempty"`
	IsCurrent       bool       `gorm:"not null" json:"is_current"`
}

func (IndexProjectionStatus) TableName() string { return "index_projection_status" }
