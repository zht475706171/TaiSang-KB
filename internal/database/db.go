package database

import (
	"fmt"

	"github.com/zht475706171/TaiSang-KB/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`).Error; err != nil {
		return fmt.Errorf("create extension vector: %w", err)
	}

	if err := db.AutoMigrate(
		&models.KnowledgeBase{},
		&models.Document{},
		&models.Chunk{},
		&models.ChatSession{},
		&models.ChatMessage{},
		&models.Setting{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	if !db.Migrator().HasColumn(&models.Chunk{}, "embedding") {
		if err := db.Exec(`ALTER TABLE chunk ADD COLUMN embedding vector(1536)`).Error; err != nil {
			return fmt.Errorf("add embedding column: %w", err)
		}
	}

	if err := db.Exec(`CREATE INDEX IF NOT EXISTS chunk_embedding_hnsw ON chunk USING hnsw (embedding vector_cosine_ops)`).Error; err != nil {
		return fmt.Errorf("create hnsw index: %w", err)
	}

	var s models.Setting
	if err := db.First(&s, 1).Error; err != nil {
		s = models.Setting{
			ID:             1,
			APIBaseURL:     "",
			APIKey:         "",
			ChatModel:      "",
			EmbeddingModel: "",
			RerankEnabled:  false,
			TopK:           4,
		}
		if err := db.Create(&s).Error; err != nil {
			return fmt.Errorf("create default setting: %w", err)
		}
	}
	return nil
}