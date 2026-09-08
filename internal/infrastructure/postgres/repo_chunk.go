package postgres

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"gorm.io/gorm"
)

type ChunkRepository struct {
	db *gorm.DB
}

func NewChunkRepository(db *gorm.DB) *ChunkRepository {
	return &ChunkRepository{db: db}
}

// InsertBatchWithEmbedding inserts chunks and their embeddings in a transaction.
// Embedding dimension must match the column dimension (1536).
func (r *ChunkRepository) InsertBatchWithEmbedding(chunks []models.Chunk, embeddings [][]float32) error {
	if len(chunks) != len(embeddings) {
		return fmt.Errorf("chunks (%d) and embeddings (%d) length mismatch", len(chunks), len(embeddings))
	}
	tx := r.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	for i, c := range chunks {
		vec := pgvector.NewVector(embeddings[i])
		if err := tx.Exec(`
			INSERT INTO chunk (id, doc_id, content, parent_content, chunk_index, token_count, embedding_retry, created_at, embedding)
			VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), ?)`,
			c.ID, c.DocID, c.Content, c.ParentContent, c.ChunkIndex, c.TokenCount, c.EmbeddingRetry, vec,
		).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("insert chunk %d: %w", i, err)
		}
	}
	return tx.Commit().Error
}

// ChunkSearchResult is a chunk hit with its score and parent content.
type ChunkSearchResult struct {
	ID            uuid.UUID
	DocID         uuid.UUID
	Content       string
	ParentContent string
	Score         float64
}

// SearchByVector returns top-k chunks by cosine similarity within the given KB.
// Only searches documents with status='done' and non-null embeddings.
func (r *ChunkRepository) SearchByVector(query []float32, kbID uuid.UUID, topK int) ([]ChunkSearchResult, error) {
	vec := pgvector.NewVector(query)
	var results []ChunkSearchResult
	err := r.db.Raw(`
		SELECT c.id, c.doc_id, c.content, c.parent_content,
		       1 - (c.embedding <=> ?) AS score
		FROM chunk c
		JOIN document d ON d.id = c.doc_id
		WHERE d.kb_id = ? AND d.status = 'done' AND c.embedding IS NOT NULL
		ORDER BY c.embedding <=> ?
		LIMIT ?`,
		vec, kbID, vec, topK,
	).Scan(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

// DeleteByDoc removes all chunks of a document.
func (r *ChunkRepository) DeleteByDoc(docID uuid.UUID) error {
	return r.db.Where("doc_id = ?", docID).Delete(&models.Chunk{}).Error
}

// CountByDoc returns chunk count for a document.
func (r *ChunkRepository) CountByDoc(docID uuid.UUID) (int64, error) {
	var n int64
	err := r.db.Model(&models.Chunk{}).Where("doc_id = ?", docID).Count(&n).Error
	return n, err
}

// ListRetryNeeded returns chunks with embedding_retry=true and null embedding.
func (r *ChunkRepository) ListRetryNeeded(limit int) ([]models.Chunk, error) {
	var chunks []models.Chunk
	err := r.db.Where("embedding_retry = ? AND embedding IS NULL", true).Limit(limit).Find(&chunks).Error
	return chunks, err
}

// UpdateEmbedding sets the embedding for a chunk and clears retry flag.
func (r *ChunkRepository) UpdateEmbedding(id uuid.UUID, vec []float32) error {
	v := pgvector.NewVector(vec)
	return r.db.Exec(`UPDATE chunk SET embedding = ?, embedding_retry = false WHERE id = ?`, v, id).Error
}