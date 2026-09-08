package postgres

import (
	"os"
	"testing"

	"github.com/zht475706171/TaiSang-KB/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TSK_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://tsk:tsk@localhost:5433/taisang_test?sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("pg not available: %v", err)
	}
	db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`)
	if err := db.AutoMigrate(&models.KnowledgeBase{}, &models.Document{}, &models.Chunk{}, &models.ChatSession{}, &models.ChatMessage{}, &models.Setting{}); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&models.Chunk{}, "embedding") {
		db.Exec(`ALTER TABLE chunk ADD COLUMN embedding vector(1536)`)
	}
	db.Exec(`CREATE INDEX IF NOT EXISTS chunk_embedding_hnsw ON chunk USING hnsw (embedding vector_cosine_ops)`)
	db.Exec(`TRUNCATE chunk, document, knowledge_base, chat_message, chat_session, setting CASCADE`)
	return db
}

// OpenTestDB is shared by all postgres/service tests.
func OpenTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return setupTestDB(t)
}