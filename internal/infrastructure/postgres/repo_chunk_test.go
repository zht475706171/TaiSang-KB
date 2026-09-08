package postgres

import (
	"testing"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

func makeVec(a, b, c float32) []float32 {
	v := make([]float32, 1536)
	v[0], v[1], v[2] = a, b, c
	return v
}

func TestChunkRepository_InsertAndSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	db := setupTestDB(t)

	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "t"}
	if err := db.Create(kb).Error; err != nil {
		t.Fatal(err)
	}
	doc := &models.Document{ID: uuid.New(), KBID: kb.ID, Title: "d", FilePath: "x.pdf", Status: "done"}
	if err := db.Create(doc).Error; err != nil {
		t.Fatal(err)
	}

	repo := NewChunkRepository(db)
	chunks := []models.Chunk{
		{ID: uuid.New(), DocID: doc.ID, Content: "hello world", ParentContent: "hello world", ChunkIndex: 0, TokenCount: 2},
		{ID: uuid.New(), DocID: doc.ID, Content: "second chunk", ParentContent: "second chunk", ChunkIndex: 1, TokenCount: 2},
	}
	vecs := [][]float32{makeVec(0.1, 0.2, 0.3), makeVec(0.4, 0.5, 0.6)}
	if err := repo.InsertBatchWithEmbedding(chunks, vecs); err != nil {
		t.Fatal(err)
	}

	got, err := repo.SearchByVector(makeVec(0.1, 0.2, 0.3), kb.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d results", len(got))
	}
	if got[0].Content != "hello world" {
		t.Errorf("top hit = %q, want 'hello world'", got[0].Content)
	}
}

func TestChunkRepository_DeleteByDoc(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	db := setupTestDB(t)
	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "t"}
	db.Create(kb)
	doc := &models.Document{ID: uuid.New(), KBID: kb.ID, Title: "d", FilePath: "x", Status: "done"}
	db.Create(doc)
	repo := NewChunkRepository(db)
	chunks := []models.Chunk{
		{ID: uuid.New(), DocID: doc.ID, Content: "a", ChunkIndex: 0},
	}
	if err := repo.InsertBatchWithEmbedding(chunks, [][]float32{makeVec(0.1, 0.2, 0.3)}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteByDoc(doc.ID); err != nil {
		t.Fatal(err)
	}
	got, err := repo.SearchByVector(makeVec(0.1, 0.2, 0.3), kb.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

func TestChunkRepository_CountByDoc(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	db := setupTestDB(t)
	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "t"}
	db.Create(kb)
	doc := &models.Document{ID: uuid.New(), KBID: kb.ID, Title: "d", FilePath: "x", Status: "done"}
	db.Create(doc)
	repo := NewChunkRepository(db)
	chunks := []models.Chunk{
		{ID: uuid.New(), DocID: doc.ID, Content: "a", ChunkIndex: 0},
		{ID: uuid.New(), DocID: doc.ID, Content: "b", ChunkIndex: 1},
	}
	if err := repo.InsertBatchWithEmbedding(chunks, [][]float32{makeVec(0.1, 0.2, 0.3), makeVec(0.4, 0.5, 0.6)}); err != nil {
		t.Fatal(err)
	}
	n, err := repo.CountByDoc(doc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}