# TaiSang-KB Implementation Plan (Part 4: Services + Handlers + Async worker)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire everything into services (kb/document/chat/setting) and HTTP handlers (Gin). Implement the async parse worker (in-process goroutine + channel queue). SSE chat endpoint.

**Architecture:**
- `service/` layer holds business logic, depends on `infrastructure/` (postgres repo, llmclient, parser, storage, crypto).
- `handler/` layer is thin HTTP/SSE glue, depends on `service/`.
- `worker/` package runs the async parse pipeline (parse → chunk → embed → persist).
- All wired in `router.New(db)` and `main.go`.

**Tech Stack:** Gin, GORM, pgvector-go (raw SQL for vector ops), SSE.

**Spec ref:** §5 components, §6 data flows, §7 errors, §9 API endpoints.

**Prerequisite:** Parts 1-3 complete.

---

### Task 1: Chunk repository (pgvector raw SQL)

**Files:**
- Create: `internal/infrastructure/postgres/repo_chunk.go`
- Create: `internal/infrastructure/postgres/repo_chunk_test.go`

- [ ] **Step 1: Write the failing test**

```go
package postgres

import (
	"testing"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

func TestChunkRepository_InsertAndSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	db := setupTestDB(t) // helper defined in this file (see Step 3)

	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "t"}
	db.Create(kb)
	doc := &models.Document{ID: uuid.New(), KBID: kb.ID, Title: "d", FilePath: "x.pdf", Status: "done"}
	db.Create(doc)

	repo := NewChunkRepository(db)
	chunks := []models.Chunk{
		{ID: uuid.New(), DocID: doc.ID, Content: "hello world", ParentContent: "hello world", ChunkIndex: 0, TokenCount: 2},
		{ID: uuid.New(), DocID: doc.ID, Content: "second chunk", ParentContent: "second chunk", ChunkIndex: 1, TokenCount: 2},
	}
	vecs := [][]float32{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6}}
	if err := repo.InsertBatchWithEmbedding(chunks, vecs); err != nil {
		t.Fatal(err)
	}

	got, err := repo.SearchByVector([]float32{0.1, 0.2, 0.3}, kb.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d results", len(got))
	}
	if got[0].Content != "hello world" {
		t.Errorf("top hit = %q", got[0].Content)
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
	if err := repo.InsertBatchWithEmbedding(chunks, [][]float32{{0.1, 0.2, 0.3}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteByDoc(doc.ID); err != nil {
		t.Fatal(err)
	}
	got, err := repo.SearchByVector([]float32{0.1, 0.2, 0.3}, kb.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/infrastructure/postgres/ -v
```
Expected: FAIL (no repo_chunk.go, no setupTestDB).

- [ ] **Step 3: Write implementation + helper**

Create `internal/infrastructure/postgres/testutil_test.go`:
```go
package postgres

import (
	"fmt"
	"os"
	"testing"

	"github.com/pgvector/pgvector-go"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TSK_TEST_DSN")
	if dsn == "" {
		dsn = "postgres://tsk:tsk@localhost:5432/taisang_test?sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("pg not available: %v", err)
	}
	db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`)
	pgvector.RegisterVector(db)
	if err := db.AutoMigrate(&models.KnowledgeBase{}, &models.Document{}, &models.Chunk{}, &models.ChatSession{}, &models.ChatMessage{}, &models.Setting{}); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasColumn(&models.Chunk{}, "embedding") {
		db.Exec(`ALTER TABLE chunk ADD COLUMN embedding vector(1536)`)
	}
	db.Exec(`TRUNCATE chunk, document, knowledge_base, chat_message, chat_session CASCADE`)
	return db
}
```

Create `internal/infrastructure/postgres/repo_chunk.go`:
```go
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

// InsertBatchWithEmbedding inserts chunks and their embeddings.
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

// ChunkSearchResult is a chunk hit with its score and parent.
type ChunkSearchResult struct {
	ID            uuid.UUID
	DocID         uuid.UUID
	Content       string
	ParentContent string
	Score         float64
}

// SearchByVector returns top-k chunks by cosine similarity within the given KB.
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

// ListRetryNeeded returns chunk IDs with embedding_retry=true.
func (r *ChunkRepository) ListRetryNeeded(limit int) ([]models.Chunk, error) {
	var chunks []models.Chunk
	err := r.db.Where("embedding_retry = ? AND embedding IS NULL", true).Limit(limit).Find(&chunks).Error
	return chunks, err
}
```

- [ ] **Step 4: Run test to verify it passes**

Start test PG:
```bash
docker run -d --name tsk-pgtest -e POSTGRES_PASSWORD=tsk -e POSTGRES_USER=tsk -e POSTGRES_DB=taisang_test -p 5432:5432 pgvector/pgvector:pg14
```
Wait ~5s, then:
```bash
go test ./internal/infrastructure/postgres/ -v
```
Expected: 2 tests PASS.

Cleanup:
```bash
docker rm -f tsk-pgtest
```

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/postgres/
git commit -m "feat(repo): chunk repository with pgvector cosine search"
```

---

### Task 2: KB / Document / Chat / Setting repositories

**Files:**
- Create: `internal/infrastructure/postgres/repo_kb.go`
- Create: `internal/infrastructure/postgres/repo_document.go`
- Create: `internal/infrastructure/postgres/repo_chat.go`
- Create: `internal/infrastructure/postgres/repo_setting.go`

- [ ] **Step 1: Write repo_kb.go**

```go
package postgres

import (
	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"gorm.io/gorm"
)

type KBRepository struct{ db *gorm.DB }

func NewKBRepository(db *gorm.DB) *KBRepository { return &KBRepository{db: db} }

func (r *KBRepository) Create(kb *models.KnowledgeBase) error {
	return r.db.Create(kb).Error
}

func (r *KBRepository) List() ([]models.KnowledgeBase, error) {
	var kbs []models.KnowledgeBase
	err := r.db.Order("created_at DESC").Find(&kbs).Error
	return kbs, err
}

func (r *KBRepository) Get(id uuid.UUID) (*models.KnowledgeBase, error) {
	var kb models.KnowledgeBase
	err := r.db.First(&kb, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &kb, err
}

func (r *KBRepository) Update(kb *models.KnowledgeBase) error {
	return r.db.Save(kb).Error
}

func (r *KBRepository) Delete(id uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete chunks of all docs in this KB.
		if err := tx.Exec(`DELETE FROM chunk WHERE doc_id IN (SELECT id FROM document WHERE kb_id = ?)`, id).Error; err != nil {
			return err
		}
		if err := tx.Where("kb_id = ?", id).Delete(&models.Document{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.KnowledgeBase{}, "id = ?", id).Error
	})
}
```

- [ ] **Step 2: Write repo_document.go**

```go
package postgres

import (
	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"gorm.io/gorm"
)

type DocumentRepository struct{ db *gorm.DB }

func NewDocumentRepository(db *gorm.DB) *DocumentRepository { return &DocumentRepository{db: db} }

func (r *DocumentRepository) Create(d *models.Document) error { return r.db.Create(d).Error }

func (r *DocumentRepository) Get(id uuid.UUID) (*models.Document, error) {
	var d models.Document
	err := r.db.First(&d, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &d, err
}

func (r *DocumentRepository) ListByKB(kbID uuid.UUID) ([]models.Document, error) {
	var docs []models.Document
	err := r.db.Where("kb_id = ?", kbID).Order("created_at DESC").Find(&docs).Error
	return docs, err
}

func (r *DocumentRepository) UpdateStatus(id uuid.UUID, status, errMsg string) error {
	return r.db.Model(&models.Document{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "error_msg": errMsg}).Error
}

func (r *DocumentRepository) Delete(id uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("doc_id = ?", id).Delete(&models.Chunk{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Document{}, "id = ?", id).Error
	})
}

func (r *DocumentRepository) ListByStatus(status string) ([]models.Document, error) {
	var docs []models.Document
	err := r.db.Where("status = ?", status).Find(&docs).Error
	return docs, err
}
```

- [ ] **Step 3: Write repo_chat.go**

```go
package postgres

import (
	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"gorm.io/gorm"
)

type ChatRepository struct{ db *gorm.DB }

func NewChatRepository(db *gorm.DB) *ChatRepository { return &ChatRepository{db: db} }

func (r *ChatRepository) CreateSession(s *models.ChatSession) error { return r.db.Create(s).Error }

func (r *ChatRepository) ListSessions(limit int) ([]models.ChatSession, error) {
	var ss []models.ChatSession
	err := r.db.Order("updated_at DESC").Limit(limit).Find(&ss).Error
	return ss, err
}

func (r *ChatRepository) GetSession(id uuid.UUID) (*models.ChatSession, error) {
	var s models.ChatSession
	err := r.db.First(&s, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &s, err
}

func (r *ChatRepository) UpdateSessionTitle(id uuid.UUID, title string) error {
	return r.db.Model(&models.ChatSession{}).Where("id = ?", id).Update("title", title).Error
}

func (r *ChatRepository) DeleteSession(id uuid.UUID) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("session_id = ?", id).Delete(&models.ChatMessage{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.ChatSession{}, "id = ?", id).Error
	})
}

func (r *ChatRepository) AddMessage(m *models.ChatMessage) error { return r.db.Create(m).Error }

func (r *ChatRepository) ListMessages(sessionID uuid.UUID, limit int) ([]models.ChatMessage, error) {
	var ms []models.ChatMessage
	err := r.db.Where("session_id = ?", sessionID).Order("created_at ASC").Limit(limit).Find(&ms).Error
	return ms, err
}

func (r *ChatRepository) RecentMessages(sessionID uuid.UUID, n int) ([]models.ChatMessage, error) {
	var ms []models.ChatMessage
	err := r.db.Where("session_id = ?", sessionID).Order("created_at DESC").Limit(n).Find(&ms).Error
	if err != nil {
		return nil, err
	}
	// reverse to chronological
	for i, j := 0, len(ms)-1; i < j; i, j = i+1, j-1 {
		ms[i], ms[j] = ms[j], ms[i]
	}
	return ms, nil
}
```

- [ ] **Step 4: Write repo_setting.go**

```go
package postgres

import (
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"gorm.io/gorm"
)

type SettingRepository struct{ db *gorm.DB }

func NewSettingRepository(db *gorm.DB) *SettingRepository { return &SettingRepository{db: db} }

func (r *SettingRepository) Get() (*models.Setting, error) {
	var s models.Setting
	err := r.db.First(&s, 1).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &s, err
}

func (r *SettingRepository) Update(s *models.Setting) error {
	return r.db.Save(s).Error
}
```

- [ ] **Step 5: Verify build**

Run:
```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/postgres/
git commit -m "feat(repo): kb/document/chat/setting repositories"
```

---

### Task 3: Setting service (with crypto)

**Files:**
- Create: `internal/service/setting/setting.go`
- Create: `internal/service/setting/setting_test.go`

- [ ] **Step 1: Write the failing test**

```go
package setting

import (
	"os"
	"testing"

	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/crypto"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

func TestSetting_GetMasksAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t) // helper to add; see Step 3
	c, _ := crypto.New("0123456789abcdef0123456789abcdef")
	svc := NewService(postgres.NewSettingRepository(db), c)

	// Seed
	svc.Update(&models.Setting{
		ID: 1, APIBaseURL: "https://api.x.com", APIKey: "sk-secret123456",
		ChatModel: "gpt-4o-mini", EmbeddingModel: "text-embedding-3-small",
		RerankEnabled: false, TopK: 4,
	})

	got, err := svc.GetMasked()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "****3456" {
		t.Errorf("masked key = %q", got.APIKey)
	}

	plain, err := svc.GetDecrypted()
	if err != nil {
		t.Fatal(err)
	}
	if plain.APIKey != "sk-secret123456" {
		t.Errorf("decrypted = %q", plain.APIKey)
	}
}
```

Add `OpenTestDB` helper to `postgres/testutil_test.go` (rename from `setupTestDB`):
```go
// OpenTestDB is shared by all postgres/service tests.
func OpenTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return setupTestDB(t)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/service/setting/ -v
```
Expected: FAIL.

- [ ] **Step 3: Write implementation**

```go
package setting

import (
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/crypto"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

type Service struct {
	repo   *postgres.SettingRepository
	crypto *crypto.Crypto
}

func NewService(repo *postgres.SettingRepository, c *crypto.Crypto) *Service {
	return &Service{repo: repo, crypto: c}
}

// GetMasked returns settings with API Key masked (****last4).
func (s *Service) GetMasked() (*models.Setting, error) {
	got, err := s.repo.Get()
	if err != nil {
		return nil, err
	}
	if got == nil {
		return &models.Setting{ID: 1, TopK: 4}, nil
	}
	got.APIKey = crypto.Mask(got.APIKey) // assume stored encrypted; mask the encrypted string for display
	return got, nil
}

// GetDecrypted returns settings with API Key decrypted.
func (s *Service) GetDecrypted() (*models.Setting, error) {
	got, err := s.repo.Get()
	if err != nil || got == nil {
		return got, err
	}
	if got.APIKey == "" {
		return got, nil
	}
	plain, err := s.crypto.Decrypt(got.APIKey)
	if err != nil {
		return nil, err
	}
	got.APIKey = plain
	return got, nil
}

// Update encrypts the API Key before persisting.
func (s *Service) Update(input *models.Setting) error {
	cur, err := s.repo.Get()
	if err != nil {
		return err
	}
	if cur == nil {
		cur = &models.Setting{ID: 1}
	}
	cur.APIBaseURL = input.APIBaseURL
	cur.ChatModel = input.ChatModel
	cur.EmbeddingModel = input.EmbeddingModel
	cur.RerankEnabled = input.RerankEnabled
	cur.TopK = input.TopK
	// Only re-encrypt when caller provides a non-masked key (i.e., not "****....").
	if input.APIKey != "" && !startsWithMask(input.APIKey) {
		enc, err := s.crypto.Encrypt(input.APIKey)
		if err != nil {
			return err
		}
		cur.APIKey = enc
	}
	return s.repo.Update(cur)
}

func startsWithMask(s string) bool {
	return len(s) >= 4 && s[:4] == "****"
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/service/setting/ -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/setting/ internal/infrastructure/postgres/testutil_test.go
git commit -m "feat(service): setting service with api key encrypt/decrypt/mask"
```

---

### Task 4: KB service

**Files:**
- Create: `internal/service/kb/kb.go`
- Create: `internal/service/kb/kb_test.go`

- [ ] **Step 1: Write the failing test**

```go
package kb

import (
	"testing"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

func TestKB_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)
	svc := NewService(postgres.NewKBRepository(db), postgres.NewDocumentRepository(db))

	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "Test KB", Description: "d"}
	if err := svc.Create(kb); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(kb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Test KB" {
		t.Errorf("name = %s", got.Name)
	}
	list, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("list len = %d", len(list))
	}
	if err := svc.Delete(kb.ID); err != nil {
		t.Fatal(err)
	}
	got, err = svc.Get(kb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil after delete")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/service/kb/ -v
```
Expected: FAIL.

- [ ] **Step 3: Write implementation**

```go
package kb

import (
	"errors"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

type Service struct {
	kbRepo   *postgres.KBRepository
	docRepo  *postgres.DocumentRepository
}

func NewService(kbRepo *postgres.KBRepository, docRepo *postgres.DocumentRepository) *Service {
	return &Service{kbRepo: kbRepo, docRepo: docRepo}
}

func (s *Service) Create(kb *models.KnowledgeBase) error {
	if kb.Name == "" {
		return errors.New("name is required")
	}
	return s.kbRepo.Create(kb)
}

func (s *Service) List() ([]models.KnowledgeBase, error) { return s.kbRepo.List() }

func (s *Service) Get(id uuid.UUID) (*models.KnowledgeBase, error) { return s.kbRepo.Get(id) }

func (s *Service) Update(kb *models.KnowledgeBase) error {
	if kb.Name == "" {
		return errors.New("name is required")
	}
	return s.kbRepo.Update(kb)
}

func (s *Service) Delete(id uuid.UUID) error { return s.kbRepo.Delete(id) }
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/service/kb/ -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/kb/
git commit -m "feat(service): kb crud"
```

---

### Task 5: Document service + async parse worker

**Files:**
- Create: `internal/service/document/document.go`
- Create: `internal/worker/parse_worker.go`
- Create: `internal/worker/parse_worker_test.go`

- [ ] **Step 1: Write the worker failing test**

```go
package worker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/llmclient"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/storage"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"github.com/zht475706171/TaiSang-KB/internal/service/chunker"
)

func TestParseWorker_PipelineEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)

	// Fake embedding client returning 3-dim vectors.
	embedSrv := startFakeEmbed(t)
	defer embedSrv.Close()
	embedC := llmclient.NewEmbeddingClient(embedSrv.URL, "sk-test", "test-model", 64)

	tmp := t.TempDir()
	st := storage.New(tmp)

	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "kb"}
	db.Create(kb)

	// Write a tiny file.
	body := []byte("# Hello\n\nWorld content here.")
	path := filepath.Join(tmp, "input.md")
	os.WriteFile(path, body, 0o644)

	doc := &models.Document{
		ID: uuid.New(), KBID: kb.ID, Title: "input.md",
		FilePath: "input.md", MimeType: "text/markdown", Status: "pending",
	}
	// Store via storage to get relative path.
	rel, _ := st.Save("input.md", "text/markdown", body)
	doc.FilePath = rel
	db.Create(doc)

	w := NewParseWorker(db, st, embedC, chunker.Split)
	w.ProcessOnce(doc.ID)

	var got models.Document
	db.First(&got, doc.ID)
	if got.Status != "done" {
		t.Fatalf("status = %s, err = %s", got.Status, got.ErrorMsg)
	}
	var n int64
	db.Model(&models.Chunk{}).Where("doc_id = ?", doc.ID).Count(&n)
	if n == 0 {
		t.Errorf("no chunks created")
	}
}

// startFakeEmbed returns an httptest server serving /v1/embeddings.
func startFakeEmbed(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llmclient.EmbedRequest
		json.NewDecoder(r.Body).Decode(&req)
		resp := llmclient.EmbedResponse{Data: make([]llmclient.EmbedItem, len(req.Input))}
		for i := range req.Input {
			resp.Data[i].Embedding = []float32{0.1, 0.2, 0.3}
			resp.Data[i].Index = i
		}
		json.NewEncoder(w).Encode(resp)
	}))
}
```
Add imports `encoding/json`, `net/http`, `net/http/httptest` at top.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/worker/ -v
```
Expected: FAIL.

- [ ] **Step 3: Write document service**

`internal/service/document/document.go`:
```go
package document

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/storage"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

const MaxFileSize = 50 * 1024 * 1024 // 50 MB

var allowedExts = map[string]bool{
	".md": true, ".markdown": true, ".txt": true,
	".html": true, ".htm": true,
	".pdf": true,
	".docx": true, ".xlsx": true, ".pptx": true,
}

type Service struct {
	docRepo  *postgres.DocumentRepository
	storage  *storage.Storage
}

func NewService(docRepo *postgres.DocumentRepository, st *storage.Storage) *Service {
	return &Service{docRepo: docRepo, storage: st}
}

// Upload reads a multipart file header, persists it, and creates a pending Document.
// Caller is expected to enqueue the doc ID into the parse worker afterwards.
func (s *Service) Upload(kbID uuid.UUID, fh *multipart.FileHeader) (*models.Document, error) {
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if !allowedExts[ext] {
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
	if fh.Size > MaxFileSize {
		return nil, errors.New("file exceeds 50MB limit")
	}
	src, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()
	body, err := io.ReadAll(src)
	if err != nil {
		return nil, err
	}
	rel, err := s.storage.Save(fh.Filename, fh.Header.Get("Content-Type"), body)
	if err != nil {
		return nil, err
	}
	doc := &models.Document{
		ID:       uuid.New(),
		KBID:     kbID,
		Title:    fh.Filename,
		Source:   "upload",
		FilePath: rel,
		MimeType: fh.Header.Get("Content-Type"),
		Status:   "pending",
		Meta:     []byte(fmt.Sprintf(`{"original_name":%q,"size":%d,"uploaded_at":%q}`, fh.Filename, fh.Size, time.Now().Format(time.RFC3339))),
	}
	if err := s.docRepo.Create(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *Service) Get(id uuid.UUID) (*models.Document, error) { return s.docRepo.Get(id) }

func (s *Service) ListByKB(kbID uuid.UUID) ([]models.Document, error) {
	return s.docRepo.ListByKB(kbID)
}

func (s *Service) Delete(id uuid.UUID) error { return s.docRepo.Delete(id) }

func (s *Service) MarkPending(id uuid.UUID) error {
	return s.docRepo.UpdateStatus(id, "pending", "")
}
```

- [ ] **Step 4: Write parse worker**

`internal/worker/parse_worker.go`:
```go
package worker

import (
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
	db       *gorm.DB
	docRepo  *postgres.DocumentRepository
	chunkRepo *postgres.ChunkRepository
	storage  *storage.Storage
	embed    *llmclient.EmbeddingClient
	chunkFn  ChunkFunc
	queue    chan uuid.UUID
	wg       sync.WaitGroup
}

func NewParseWorker(db *gorm.DB, st *storage.Storage, embed *llmclient.EmbeddingClient, chunkFn ChunkFunc) *ParseWorker {
	w := &ParseWorker{
		db:        db,
		docRepo:   postgres.NewDocumentRepository(db),
		chunkRepo: postgres.NewChunkRepository(db),
		storage:   st,
		embed:     embed,
		chunkFn:   chunkFn,
		queue:     make(chan uuid.UUID, 100),
	}
	return w
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
		// Clean any partial chunks for parsing docs.
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
	// Derive original name from meta if needed; for parse we just need extension.
	filename := filepath.Base(doc.FilePath)
	text, err := parser.Parse(body, filename, doc.MimeType)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	chunks, err := w.chunkFn(text, chunker.SplitOptions{
		ParentTarget: 800,
		ChildTarget:  256,
		ChildOverlap: 50,
	})
	if err != nil {
		return fmt.Errorf("chunk: %w", err)
	}
	if len(chunks) == 0 {
		return nil // nothing to embed
	}
	contents := make([]string, len(chunks))
	for i, c := range chunks {
		contents[i] = c.Content
	}
	vecs, err := w.embed.EmbedBatch(contents)
	if err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	// Build models.Chunk slice.
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
func (w *ParseWorker) EmbedRetryLoop(ctx interface{ Done() <-chan struct{} }) {
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
	// Update each chunk embedding via direct SQL.
	for i, c := range chunks {
		_ = w.db.Exec(`UPDATE chunk SET embedding = ?, embedding_retry = false WHERE id = ?`,
			pgvectorNewVector(vecs[i]), c.ID).Error
	}
}
```

Note: `pgvectorNewVector` is a small wrapper to avoid importing pgvector in this file's import list multiple times. Add at top of file:
```go
import (
	// ...
	"github.com/pgvector/pgvector-go"
)

func pgvectorNewVector(v []float32) pgvector.Vector {
	return pgvector.NewVector(v)
}
```

- [ ] **Step 5: Run test to verify it passes**

Start test PG (same as Task 1):
```bash
docker run -d --name tsk-pgtest -e POSTGRES_USER=tsk -e POSTGRES_PASSWORD=tsk -e POSTGRES_DB=taisang_test -p 5432:5432 pgvector/pgvector:pg14
```
Then:
```bash
go test ./internal/worker/ -v
```
Expected: PASS (document status=done, chunks created).

Cleanup:
```bash
docker rm -f tsk-pgtest
```

- [ ] **Step 6: Commit**

```bash
git add internal/service/document/ internal/worker/
git commit -m "feat(worker): async parse pipeline (parse→chunk→embed→persist) + retry loop"
```

---

### Task 6: Chat service (RAG)

**Files:**
- Create: `internal/service/chat/chat.go`
- Create: `internal/service/chat/chat_test.go`

- [ ] **Step 1: Write the failing test**

```go
package chat

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/llmclient"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

func TestChat_RAGStream(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)

	// Seed KB + doc + chunk with embedding.
	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "kb"}
	db.Create(kb)
	doc := &models.Document{ID: uuid.New(), KBID: kb.ID, Title: "d", FilePath: "x", Status: "done"}
	db.Create(doc)
	chunkRepo := postgres.NewChunkRepository(db)
	chunkRepo.InsertBatchWithEmbedding(
		[]models.Chunk{{ID: uuid.New(), DocID: doc.ID, Content: "地球是圆的。", ParentContent: "地球是圆的。", ChunkIndex: 0, TokenCount: 5}},
		[][]float32{{0.1, 0.2, 0.3}},
	)

	// Fake LLM streams "Answer: 地球是圆的".
	chatSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *r.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		f.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Answer: \"}}]}\n\n"); f.Flush()
		f.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"地球是圆的\"}}]}\n\n"); f.Flush()
		f.Fprintf(w, "data: [DONE]\n\n"); f.Flush()
	}))
	defer chatSrv.Close()

	embedSrv := startFakeEmbed(t)
	defer embedSrv.Close()

	settingSvc := newMockSetting("https://nope", "sk-test", "gpt-4o-mini", "test-model", false, 4)
	svc := NewService(
		postgres.NewChatRepository(db),
		postgres.NewChunkRepository(db),
		settingSvc,
		llmclient.NewEmbeddingClient(embedSrv.URL, "sk-test", "test-model", 64),
		llmclient.NewChatClient(chatSrv.URL, "sk-test", "gpt-4o-mini"),
		nil, // no rerank
	)

	var sb strings.Builder
	citations, err := svc.Chat(kb.ID, "地球什么形状？", uuid.Nil, func(delta string) error {
		sb.WriteString(delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "地球是圆的") {
		t.Errorf("answer = %q", sb.String())
	}
	if len(citations) == 0 {
		t.Errorf("expected citations")
	}
}

// helpers below
type mockSetting struct {
	base, key, chat, embed string
	rerank                 bool
	topK                   int
}

func newMockSetting(base, key, chat, embed string, rerank bool, topK int) *mockSetting {
	return &mockSetting{base, key, chat, embed, rerank, topK}
}
func (m *mockSetting) GetDecrypted() (*models.Setting, error) {
	return &models.Setting{ID: 1, APIBaseURL: m.base, APIKey: m.key, ChatModel: m.chat, EmbeddingModel: m.embed, RerankEnabled: m.rerank, TopK: m.topK}, nil
}
```
Add missing imports `"net/http"`, `"net/http/httptest"` and fix the typo `r.Request` → `http.Request`. Also add `startFakeEmbed` helper same as in worker test.

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/service/chat/ -v
```
Expected: FAIL.

- [ ] **Step 3: Write implementation**

```go
package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/llmclient"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

// SettingProvider is the subset of setting.Service the chat service needs.
type SettingProvider interface {
	GetDecrypted() (*models.Setting, error)
}

type Citation struct {
	ChunkID    uuid.UUID `json:"chunk_id"`
	DocID      uuid.UUID `json:"doc_id"`
	DocTitle   string    `json:"doc_title"`
	Snippet    string    `json:"content_snippet"`
	Score      float64   `json:"score"`
}

type Service struct {
	chatRepo  *postgres.ChatRepository
	chunkRepo *postgres.ChunkRepository
	setting   SettingProvider
	embed     *llmclient.EmbeddingClient
	chat      *llmclient.ChatClient
	rerank    *llmclient.RerankClient
}

func NewService(
	chatRepo *postgres.ChatRepository,
	chunkRepo *postgres.ChunkRepository,
	setting SettingProvider,
	embed *llmclient.EmbeddingClient,
	chat *llmclient.ChatClient,
	rerank *llmclient.RerankClient,
) *Service {
	return &Service{chatRepo, chunkRepo, setting, embed, chat, rerank}
}

// Chat runs RAG: embed question → retrieve → (rerank) → stream LLM answer.
// onDelta is called for each token. Returns citations (final frame).
// If sessionID is zero, a new session is created.
func (s *Service) Chat(kbID uuid.UUID, question string, sessionID uuid.UUID, onDelta func(string) error) ([]Citation, error) {
	if strings.TrimSpace(question) == "" {
		return nil, errors.New("empty question")
	}
	setting, err := s.setting.GetDecrypted()
	if err != nil || setting == nil {
		return nil, fmt.Errorf("setting: %w", err)
	}
	if setting.APIKey == "" || setting.ChatModel == "" {
		return nil, errors.New("api_key or chat_model not configured")
	}

	// 1. Embed question.
	qvecs, err := s.embed.EmbedBatch([]string{question})
	if err != nil {
		return nil, fmt.Errorf("embed question: %w", err)
	}
	if len(qvecs) == 0 {
		return nil, errors.New("empty embedding")
	}

	// 2. Retrieve.
	hits, err := s.chunkRepo.SearchByVector(qvecs[0], kbID, 20)
	if err != nil {
		return nil, fmt.Errorf("retrieve: %w", err)
	}
	if len(hits) == 0 {
		// No hits → fallback answer, no LLM call.
		fallback := "知识库中未找到相关内容，请尝试换种问法或上传更多文档。"
		_ = onDelta(fallback)
		return nil, s.persistTurn(sessionID, kbID, question, fallback, nil)
	}

	// 3. Rerank (optional).
	topK := setting.TopK
	if topK <= 0 {
		topK = 4
	}
	if s.rerank != nil && setting.RerankEnabled {
		docs := make([]string, len(hits))
		for i, h := range hits {
			docs[i] = h.Content
		}
		rr, err := s.rerank.Rerank(question, docs)
		if err == nil && len(rr) > 0 {
			// Reorder hits per rerank results.
			reordered := make([]postgres.ChunkSearchResult, len(rr))
			for i, item := range rr {
				if item.Index >= 0 && item.Index < len(hits) {
					reordered[i] = hits[item.Index]
					reordered[i].Score = item.RelevanceScore
				}
			}
			hits = reordered
		}
	}
	if len(hits) > topK {
		hits = hits[:topK]
	}

	// 4. Build citations + context (parent content).
	citations := make([]Citation, len(hits))
	var contextBuf strings.Builder
	for i, h := range hits {
		title := ""
		var doc models.Document
		if err := s.chatRepo.GetDB().First(&doc, "id = ?", h.DocID).Error; err == nil {
			title = doc.Title
		}
		citations[i] = Citation{
			ChunkID: h.ID, DocID: h.DocID, DocTitle: title,
			Snippet: truncate(h.Content, 200), Score: h.Score,
		}
		contextBuf.WriteString(h.ParentContent)
		contextBuf.WriteString("\n\n")
	}

	// 5. Build messages.
	system := "你是知识库问答助手。根据以下检索到的内容回答用户问题。如果检索内容不足以回答，请如实说明。"
	retrieved := fmt.Sprintf("检索到的内容：\n%s", contextBuf.String())
	msgs := []llmclient.ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: retrieved + "\n\n用户问题：" + question},
	}

	// 6. Stream.
	var answer strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx
	streamErr := s.chat.StreamChat(msgs, func(delta string) error {
		answer.WriteString(delta)
		return onDelta(delta)
	})
	if streamErr != nil {
		// Persist partial answer.
		_ = s.persistTurn(sessionID, kbID, question, answer.String(), citations)
		return citations, streamErr
	}

	// 7. Persist.
	if err := s.persistTurn(sessionID, kbID, question, answer.String(), citations); err != nil {
		return citations, err
	}
	return citations, nil
}

func (s *Service) persistTurn(sessionID uuid.UUID, kbID uuid.UUID, question, answer string, citations []Citation) error {
	if sessionID == uuid.Nil {
		sessionID = uuid.New()
		session := &models.ChatSession{ID: sessionID, KBID: &kbID, Title: truncate(question, 40)}
		if err := s.chatRepo.CreateSession(session); err != nil {
			return err
		}
	}
	// Save user message.
	if err := s.chatRepo.AddMessage(&models.ChatMessage{
		ID: uuid.New(), SessionID: sessionID, Role: "user", Content: question,
	}); err != nil {
		return err
	}
	// Save assistant message with citations JSON.
	citJSON := citationsToJSON(citations)
	return s.chatRepo.AddMessage(&models.ChatMessage{
		ID: uuid.New(), SessionID: sessionID, Role: "assistant", Content: answer, Citations: citJSON,
	})
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func citationsToJSON(cs []Citation) []byte {
	if len(cs) == 0 {
		return []byte("[]")
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, c := range cs {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"chunk_id":%q,"doc_id":%q,"doc_title":%q,"content_snippet":%q,"score":%f}`,
			c.ChunkID, c.DocID, c.DocTitle, c.Snippet, c.Score)
	}
	b.WriteByte(']')
	return []byte(b.String())
}
```

Add `GetDB()` helper on `ChatRepository`:
```go
// in repo_chat.go
func (r *ChatRepository) GetDB() *gorm.DB { return r.db }
```

- [ ] **Step 4: Run test to verify it passes**

```bash
docker run -d --name tsk-pgtest -e POSTGRES_USER=tsk -e POSTGRES_PASSWORD=tsk -e POSTGRES_DB=taisang_test -p 5432:5432 pgvector/pgvector:pg14
go test ./internal/service/chat/ -v
docker rm -f tsk-pgtest
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/service/chat/ internal/infrastructure/postgres/repo_chat.go
git commit -m "feat(service): rag chat with retrieve/rerank/stream and citations"
```

---

### Task 7: HTTP handlers

**Files:**
- Create: `internal/handler/kb.go`
- Create: `internal/handler/document.go`
- Create: `internal/handler/chat.go`
- Create: `internal/handler/setting.go`
- Modify: `internal/router/router.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Write handler/kb.go**

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"github.com/zht475706171/TaiSang-KB/internal/service/kb"
)

type KBHandler struct {
	svc *kb.Service
}

func NewKBHandler(svc *kb.Service) *KBHandler { return &KBHandler{svc: svc} }

func (h *KBHandler) Create(c *gin.Context) {
	var body struct {
		Name           string `json:"name" binding:"required"`
		Description    string `json:"description"`
		EmbeddingModel string `json:"embedding_model"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	k := &models.KnowledgeBase{
		ID: uuid.New(), Name: body.Name, Description: body.Description, EmbeddingModel: body.EmbeddingModel,
	}
	if err := h.svc.Create(k); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, k)
}

func (h *KBHandler) List(c *gin.Context) {
	kbs, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": kbs})
}

func (h *KBHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	k, err := h.svc.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if k == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, k)
}

func (h *KBHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Name           string `json:"name"`
		Description    string `json:"description"`
		EmbeddingModel string `json:"embedding_model"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	k, err := h.svc.Get(id)
	if err != nil || k == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	k.Name = body.Name
	k.Description = body.Description
	k.EmbeddingModel = body.EmbeddingModel
	if err := h.svc.Update(k); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, k)
}

func (h *KBHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.svc.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
```

- [ ] **Step 2: Write handler/document.go**

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/service/document"
	"github.com/zht475706171/TaiSang-KB/internal/worker"
)

type DocumentHandler struct {
	svc    *document.Service
	worker *worker.ParseWorker
}

func NewDocumentHandler(svc *document.Service, w *worker.ParseWorker) *DocumentHandler {
	return &DocumentHandler{svc: svc, worker: w}
}

func (h *DocumentHandler) Upload(c *gin.Context) {
	kbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid kb id"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}
	doc, err := h.svc.Upload(kbID, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.worker.Enqueue(doc.ID)
	c.JSON(http.StatusAccepted, doc)
}

func (h *DocumentHandler) List(c *gin.Context) {
	kbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid kb id"})
		return
	}
	docs, err := h.svc.ListByKB(kbID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": docs})
}

func (h *DocumentHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	d, err := h.svc.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, d)
}

func (h *DocumentHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.svc.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (h *DocumentHandler) Reparse(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	d, err := h.svc.Get(id)
	if err != nil || d == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err := h.svc.MarkPending(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.worker.Enqueue(id)
	c.JSON(http.StatusAccepted, gin.H{"status": "queued"})
}
```

- [ ] **Step 3: Write handler/chat.go (SSE)**

```go
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/service/chat"
)

type ChatHandler struct {
	svc *chat.Service
}

func NewChatHandler(svc *chat.Service) *ChatHandler { return &ChatHandler{svc: svc} }

func (h *ChatHandler) Chat(c *gin.Context) {
	var body struct {
		KBID      string `json:"kb_id" binding:"required"`
		Question  string `json:"question" binding:"required"`
		SessionID string `json:"session_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	kbID, err := uuid.Parse(body.KBID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid kb_id"})
		return
	}
	sessionID := uuid.Nil
	if body.SessionID != "" {
		sessionID, _ = uuid.Parse(body.SessionID)
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	citations, err := h.svc.Chat(kbID, body.Question, sessionID, func(delta string) error {
		data, _ := json.Marshal(map[string]string{"delta": delta})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
		return nil
	})
	if err != nil {
		errData, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(c.Writer, "data: %s\n\n", errData)
		flusher.Flush()
		return
	}
	citData, _ := json.Marshal(map[string]any{"citations": citations})
	fmt.Fprintf(c.Writer, "data: %s\n\n", citData)
	flusher.Flush()
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}

func (h *ChatHandler) ListSessions(c *gin.Context) {
	// delegate to chatRepo via service if added; for now return empty
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (h *ChatHandler) GetMessages(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (h *ChatHandler) DeleteSession(c *gin.Context) {
	c.JSON(http.StatusNoContent, nil)
}
```

Note: session listing/messages are stubbed here. Implementing them fully requires adding methods to chat service; for v1 the chat session is created internally and the frontend tracks session_id from a future endpoint. To keep scope tight, leave as stubs and document as known gap.

- [ ] **Step 4: Write handler/setting.go**

```go
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"github.com/zht475706171/TaiSang-KB/internal/service/setting"
)

type SettingHandler struct {
	svc *setting.Service
}

func NewSettingHandler(svc *setting.Service) *SettingHandler { return &SettingHandler{svc: svc} }

func (h *SettingHandler) Get(c *gin.Context) {
	s, err := h.svc.GetMasked()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *SettingHandler) Update(c *gin.Context) {
	var body models.Setting
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.ID = 1
	if err := h.svc.Update(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

- [ ] **Step 5: Wire router and main.go**

Replace `internal/router/router.go`:
```go
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pgvector/pgvector-go"
	"github.com/zht475706171/TaiSang-KB/internal/handler"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/crypto"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/llmclient"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/storage"
	"github.com/zht475706171/TaiSang-KB/internal/service/chat"
	"github.com/zht475706171/TaiSang-KB/internal/service/document"
	"github.com/zht475706171/TaiSang-KB/internal/service/kb"
	"github.com/zht475706171/TaiSang-KB/internal/service/setting"
	"github.com/zht475706171/TaiSang-KB/internal/worker"
	"gorm.io/gorm"
)

type Services struct {
	KB       *kb.Service
	Document *document.Service
	Chat     *chat.Service
	Setting  *setting.Service
	Worker   *worker.ParseWorker
}

func NewServices(db *gorm.DB, cfg Config) (*Services, error) {
	st := storage.New(cfg.StorageDir)
	c, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}

	settingRepo := postgres.NewSettingRepository(db)
	kbRepo := postgres.NewKBRepository(db)
	docRepo := postgres.NewDocumentRepository(db)
	chunkRepo := postgres.NewChunkRepository(db)
	chatRepo := postgres.NewChatRepository(db)

	settingSvc := setting.NewService(settingRepo, c)
	// Initialize default LLM clients from current settings (will be used lazily).
	s, _ := settingSvc.GetDecrypted()
	var embedC *llmclient.EmbeddingClient
	var chatC *llmclient.ChatClient
	var rerankC *llmclient.RerankClient
	if s != nil && s.APIKey != "" {
		embedC = llmclient.NewEmbeddingClient(s.APIBaseURL, s.APIKey, s.EmbeddingModel, 64)
		chatC = llmclient.NewChatClient(s.APIBaseURL, s.APIKey, s.ChatModel)
		// rerankC left nil for v1; enable later.
	}

	kbSvc := kb.NewService(kbRepo, docRepo)
	docSvc := document.NewService(docRepo, st)
	chatSvc := chat.NewService(chatRepo, chunkRepo, settingSvc, embedC, chatC, rerankC)

	w := worker.NewParseWorker(db, st, embedC, nil) // chunkFn set below
	// Use the real chunker.
	w.SetChunkFn = nil // see note
	_ = chatSvc

	return &Services{KB: kbSvc, Document: docSvc, Chat: chatSvc, Setting: settingSvc, Worker: w}, nil
}

// Config is the router's view of app config.
type Config struct {
	ListenAddr    string
	StorageDir    string
	EncryptionKey string
}

func New(db *gorm.DB, cfg Config) *gin.Engine {
	svcs, err := NewServices(db, cfg)
	if err != nil {
		panic(err)
	}
	_ = pgvector.RegisterVector // ensure import used

	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

		kbH := handler.NewKBHandler(svcs.KB)
		api.POST("/kb", kbH.Create)
		api.GET("/kb", kbH.List)
		api.GET("/kb/:id", kbH.Get)
		api.PUT("/kb/:id", kbH.Update)
		api.DELETE("/kb/:id", kbH.Delete)

		docH := handler.NewDocumentHandler(svcs.Document, svcs.Worker)
		api.POST("/kb/:id/documents", docH.Upload)
		api.GET("/kb/:id/documents", docH.List)
		api.GET("/documents/:id", docH.Get)
		api.DELETE("/documents/:id", docH.Delete)
		api.POST("/documents/:id/reparse", docH.Reparse)

		chatH := handler.NewChatHandler(svcs.Chat)
		api.POST("/chat", chatH.Chat)
		api.GET("/sessions", chatH.ListSessions)
		api.GET("/sessions/:id/messages", chatH.GetMessages)
		api.DELETE("/sessions/:id", chatH.DeleteSession)

		setH := handler.NewSettingHandler(svcs.Setting)
		api.GET("/setting", setH.Get)
		api.PUT("/setting", setH.Update)
	}

	svcs.Worker.Start()
	svcs.Worker.Recover()

	return r
}
```

Note: the worker's chunkFn signature in Part 4 Task 5 used `chunker.Split` directly. To inject cleanly, change `NewParseWorker` to accept `chunkFn ChunkFunc` instead of nil. Update:
```go
w := worker.NewParseWorker(db, st, embedC, chunker.Split)
```
Add import `"github.com/zht475706171/TaiSang-KB/internal/service/chunker"`.

Also update the router_test.go (Part 1 Task 8) — the `New(nil)` signature changed to `New(db, cfg)`. Adjust:
```go
func TestHealthEndpoint(t *testing.T) {
	r := New(nil, Config{})
	// ... rest unchanged
}
```

- [ ] **Step 6: Update main.go**

Replace `cmd/server/main.go`:
```go
package main

import (
	"log"

	"go.uber.org/zap"

	"github.com/zht475706171/TaiSang-KB/internal/config"
	"github.com/zht475706171/TaiSang-KB/internal/database"
	"github.com/zht475706171/TaiSang-KB/internal/logger"
	"github.com/zht475706171/TaiSang-KB/internal/router"
)

func main() {
	if err := logger.Init(); err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.L.Fatal("config load failed", zap.Error(err))
	}
	db, err := database.Connect(cfg.DBDSN)
	if err != nil {
		logger.L.Fatal("db connect failed", zap.Error(err))
	}
	if err := database.Migrate(db); err != nil {
		logger.L.Fatal("db migrate failed", zap.Error(err))
	}

	r := router.New(db, router.Config{
		ListenAddr:    cfg.ListenAddr,
		StorageDir:    cfg.StorageDir,
		EncryptionKey: cfg.EncryptionKey,
	})
	logger.L.Info("listening", zap.String("addr", cfg.ListenAddr))
	if err := r.Run(cfg.ListenAddr); err != nil {
		logger.L.Fatal("server stopped", zap.Error(err))
	}
}
```

- [ ] **Step 7: Verify build**

```bash
go build ./...
```
Expected: no errors. Fix any import cycles or signature mismatches.

- [ ] **Step 8: Run all tests**

```bash
go test ./... -count=1 -short
```
Expected: all unit tests PASS (integration tests skip without PG).

- [ ] **Step 9: Commit**

```bash
git add internal/handler/ internal/router/ cmd/server/main.go
git commit -m "feat(handler): wire all endpoints + services + worker into router"
```

---

### Task 8: Self-review and checkpoint

- [ ] **Step 1: Full integration smoke test**

```bash
docker compose up -d postgres
cp .env.example .env
# Edit .env: set TSK_ENCRYPTION_KEY to a 32-char string
export $(grep -v '^#' .env | xargs)
go run ./cmd/server
# In another terminal:
curl http://127.0.0.1:8080/api/health
# Expected: {"status":"ok"}
curl -X POST http://127.0.0.1:8080/api/setting -H 'Content-Type: application/json' \
  -d '{"api_base_url":"https://api.openai.com","api_key":"sk-...","chat_model":"gpt-4o-mini","embedding_model":"text-embedding-3-small","top_k":4}'
# Expected: {"status":"ok"}
curl -X POST http://127.0.0.1:8080/api/kb -H 'Content-Type: application/json' -d '{"name":"My KB"}'
# Expected: {"id":"...","name":"My KB",...}
```

- [ ] **Step 2: Run whole test suite**

```bash
go test ./... -count=1
go vet ./...
```

- [ ] **Step 3: Commit checkpoint**

```bash
git commit --allow-empty -m "checkpoint: Part 4 services+handlers+worker complete"
```

---

## End of Part 4

**What's done:**
- Chunk repository with pgvector cosine search
- KB / Document / Chat / Setting repositories
- Setting service with API key encrypt/decrypt/mask
- KB service (CRUD)
- Document service (upload with validation)
- Async parse worker (parse → chunk → embed → persist) + retry loop + recovery on restart
- Chat service (RAG: embed → retrieve → optional rerank → stream LLM → citations)
- All HTTP handlers wired
- SSE chat endpoint
- Main.go updated

**Known gaps (v1 acceptable):**
- Session listing/messages endpoints are stubs (chat session is created internally; frontend tracks from response)
- Rerank client is wired but not enabled by default
- Embedding retry loop exists but not started in main.go (add `go worker.EmbedRetryLoop(ctx)` in v1.1)

**Next:** Part 5 — Frontend (Vue 3 + TDesign, three pages).