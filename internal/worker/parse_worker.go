package worker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/llmclient"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/parser"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/storage"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"github.com/zht475706171/TaiSang-KB/internal/service/chunker"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ChunkFunc = func(string, chunker.SplitOptions) ([]chunker.Chunk, error)

type ParseWorker struct {
	db        *gorm.DB
	docRepo   *postgres.DocumentRepository
	chunkRepo *postgres.ChunkRepository
	storage   *storage.Storage
	embed     *llmclient.EmbeddingClient
	chunkFn   ChunkFunc
	queue     chan uuid.UUID
	wg        sync.WaitGroup
}

func NewParseWorker(db *gorm.DB, st *storage.Storage, embed *llmclient.EmbeddingClient, chunkFn ChunkFunc) *ParseWorker {
	return &ParseWorker{
		db:        db,
		docRepo:   postgres.NewDocumentRepository(db),
		chunkRepo: postgres.NewChunkRepository(db),
		storage:   st,
		embed:     embed,
		chunkFn:   chunkFn,
		queue:     make(chan uuid.UUID, 100),
	}
}

// Start spawns the worker goroutine. Stop with Stop().
func (w *ParseWorker) Start() {
	w.wg.Add(1)
	go w.loop()
}

func (w *ParseWorker) Stop() {
	close(w.queue)
	w.wg.Wait()
}

func (w *ParseWorker) Enqueue(docID uuid.UUID) {
	select {
	case w.queue <- docID:
	default:
		zap.L().Warn("parse queue full, dropping", zap.String("doc_id", docID.String()))
	}
}

// Recover scans pending/parsing docs and re-enqueues them. Call once at startup.
func (w *ParseWorker) Recover() {
	docs, err := w.docRepo.ListByStatus("pending")
	if err != nil {
		zap.L().Error("recover pending", zap.Error(err))
		return
	}
	parsing, _ := w.docRepo.ListByStatus("parsing")
	docs = append(docs, parsing...)
	for _, d := range docs {
		if d.Status == "parsing" {
			_ = w.chunkRepo.DeleteByDoc(d.ID)
		}
		w.Enqueue(d.ID)
	}
}

func (w *ParseWorker) loop() {
	defer w.wg.Done()
	for id := range w.queue {
		w.ProcessOnce(id)
	}
}

// ProcessOnce runs the full pipeline for one document. Exported for testing.
func (w *ParseWorker) ProcessOnce(docID uuid.UUID) {
	doc, err := w.docRepo.Get(docID)
	if err != nil || doc == nil {
		zap.L().Error("worker: doc not found", zap.String("id", docID.String()))
		return
	}
	if err := w.docRepo.UpdateStatus(docID, "parsing", ""); err != nil {
		zap.L().Error("worker: mark parsing", zap.Error(err))
		return
	}
	if err := w.process(doc); err != nil {
		w.docRepo.UpdateStatus(docID, "failed", err.Error())
		zap.L().Error("worker: process failed", zap.String("doc", docID.String()), zap.Error(err))
		return
	}
	w.docRepo.UpdateStatus(docID, "done", "")
}

func (w *ParseWorker) process(doc *models.Document) error {
	full := w.storage.FullPath(doc.FilePath)
	if full == "" {
		return fmt.Errorf("invalid file path: %s", doc.FilePath)
	}
	body, err := os.ReadFile(full)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	filename := filepath.Base(doc.FilePath)
	text, err := parser.Parse(body, filename, doc.MimeType)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	chunks, err := w.chunkFn(text, chunker.SplitOptions{
		ParentTarget: 4096,
		ChildTarget:  384,
		ChildOverlap: 76, // 76 = 384/5, WeKnora 默认 overlap 比例
	})
	if err != nil {
		return fmt.Errorf("chunk: %w", err)
	}
	if len(chunks) == 0 {
		return nil
	}
	contents := make([]string, len(chunks))
	for i, c := range chunks {
		contents[i] = c.Content
	}
	vecs, err := w.embed.EmbedBatch(contents)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	mChunks := make([]models.Chunk, len(chunks))
	for i, c := range chunks {
		mChunks[i] = models.Chunk{
			ID:            uuid.New(),
			DocID:         doc.ID,
			Content:       c.Content,
			ParentContent: c.ParentContent,
			ChunkIndex:    c.ChunkIndex,
			TokenCount:    c.TokenCount,
		}
	}
	if err := w.chunkRepo.InsertBatchWithEmbedding(mChunks, vecs); err != nil {
		return fmt.Errorf("insert chunks: %w", err)
	}
	return nil
}

// EmbedRetryLoop periodically retries chunks with embedding_retry=true.
func (w *ParseWorker) EmbedRetryLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.retryEmbeddings()
		case <-ctx.Done():
			return
		}
	}
}

func (w *ParseWorker) retryEmbeddings() {
	chunks, err := w.chunkRepo.ListRetryNeeded(100)
	if err != nil || len(chunks) == 0 {
		return
	}
	contents := make([]string, len(chunks))
	for i, c := range chunks {
		contents[i] = c.Content
	}
	vecs, err := w.embed.EmbedBatch(contents)
	if err != nil {
		zap.L().Warn("retry embeddings failed", zap.Error(err))
		return
	}
	for i, c := range chunks {
		_ = w.chunkRepo.UpdateEmbedding(c.ID, vecs[i])
	}
}