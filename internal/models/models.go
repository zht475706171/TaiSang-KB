package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// KnowledgeBase is a collection of documents.
type KnowledgeBase struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Name           string    `gorm:"size:255;not null" json:"name"`
	Description    string    `gorm:"size:1024" json:"description"`
	EmbeddingModel string    `gorm:"size:128" json:"embedding_model"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Document is an uploaded file in a KB.
type Document struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	KBID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"kb_id"`
	Title     string         `gorm:"size:512;not null" json:"title"`
	Source    string         `gorm:"size:32;not null;default:'upload'" json:"source"`
	FilePath  string         `gorm:"size:1024;not null" json:"file_path"`
	MimeType  string         `gorm:"size:128" json:"mime_type"`
	Status    string         `gorm:"size:32;not null;default:'pending';index" json:"status"`
	ErrorMsg  string         `gorm:"size:1024" json:"error_msg"`
	Meta      datatypes.JSON `gorm:"type:jsonb" json:"meta"`
	CreatedAt time.Time      `json:"created_at"`
}

// Chunk is a searchable text slice of a document.
type Chunk struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	DocID          uuid.UUID `gorm:"type:uuid;not null;index" json:"doc_id"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	ParentContent  string    `gorm:"type:text" json:"parent_content"`
	ChunkIndex     int       `gorm:"not null" json:"chunk_index"`
	TokenCount     int       `json:"token_count"`
	EmbeddingRetry bool      `gorm:"default:false" json:"embedding_retry"`
	CreatedAt      time.Time `json:"created_at"`
	// Embedding is set via raw SQL (pgvector type); not declared here to avoid driver complexity.
}

// ChatSession groups messages in one conversation.
type ChatSession struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Title     string     `gorm:"size:255" json:"title"`
	KBID      *uuid.UUID `gorm:"type:uuid;index" json:"kb_id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ChatMessage is one user or assistant turn.
type ChatMessage struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	SessionID uuid.UUID      `gorm:"type:uuid;not null;index" json:"session_id"`
	Role      string         `gorm:"size:32;not null" json:"role"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	Citations datatypes.JSON `gorm:"type:jsonb" json:"citations"`
	CreatedAt time.Time      `json:"created_at"`
}

// Setting is the single-row app settings table.
type Setting struct {
	ID             int    `gorm:"primaryKey" json:"id"`
	APIBaseURL     string `gorm:"size:512" json:"api_base_url"`
	APIKey         string `gorm:"size:1024" json:"api_key"` // encrypted
	ChatModel      string `gorm:"size:128" json:"chat_model"`
	EmbeddingModel string `gorm:"size:128" json:"embedding_model"`
	RerankEnabled  bool   `json:"rerank_enabled"`
	TopK           int    `gorm:"default:4" json:"top_k"`
}

func (KnowledgeBase) TableName() string { return "knowledge_base" }
func (Document) TableName() string      { return "document" }
func (Chunk) TableName() string         { return "chunk" }
func (ChatSession) TableName() string   { return "chat_session" }
func (ChatMessage) TableName() string   { return "chat_message" }
func (Setting) TableName() string       { return "setting" }