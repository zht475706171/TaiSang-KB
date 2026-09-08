package database

import (
	"testing"

	"github.com/zht475706171/TaiSang-KB/internal/models"
)

func TestConnectAndMigrate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dsn := "postgres://tsk:tsk@localhost:5433/taisang_test?sslmode=disable"
	db, err := Connect(dsn)
	if err != nil {
		t.Skipf("pg not available on 5433, skipping: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	tables := []string{"knowledge_base", "document", "chunk", "chat_session", "chat_message", "setting"}
	for _, tbl := range tables {
		if !db.Migrator().HasTable(tbl) {
			t.Errorf("table %s not created", tbl)
		}
	}
	// embedding column on chunk
	if !db.Migrator().HasColumn(&models.Chunk{}, "embedding") {
		t.Errorf("embedding column not created on chunk")
	}
	// HNSW index
	var idxExists bool
	db.Raw(`SELECT EXISTS(SELECT 1 FROM pg_indexes WHERE indexname = 'chunk_embedding_hnsw')`).Scan(&idxExists)
	if !idxExists {
		t.Errorf("HNSW index not created")
	}
	// settings default row
	var s models.Setting
	if err := db.First(&s, 1).Error; err != nil {
		t.Errorf("default setting row missing: %v", err)
	}
}