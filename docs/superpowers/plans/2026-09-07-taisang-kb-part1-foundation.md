# TaiSang-KB Implementation Plan (Part 1: Foundation)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a minimal personal/small-team RAG knowledge base: upload docs → parse → chunk → embed → retrieve → answer, with a Vue 3 + TDesign frontend.

**Architecture:** Single Go process (Gin + GORM) + PostgreSQL with pgvector. Async parse worker (in-process goroutine + channel queue). SSE streaming chat. No auth, binds 127.0.0.1. Vue 3 SPA frontend reusing WeKnora's visual style (TDesign Vue Next).

**Tech Stack:** Go 1.22+ / Gin / GORM / pgvector / unioffice / ledongthuc/pdf / zap. Vue 3 / TDesign Vue Next / Pinia / Vite / Less / marked / highlight.js / KaTeX / @microsoft/fetch-event-source.

**Spec:** `docs/superpowers/specs/2026-09-07-taisang-kb-design.md`

**Scope of this plan (Part 1):** Project scaffold + config + DB schema + models + health endpoint + docker compose. Subsequent parts: parser/chunker, embedding/llm, RAG/chat, frontend, polish.

---

### Task 1: Initialize Go module and project skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `Makefile`
- Create: `.gitignore`

- [ ] **Step 1: Init go module**

Run:
```bash
cd D:/Project/TaiSang-KB
go mod init github.com/zht475706171/TaiSang-KB
```
Expected: `go: creating new go.mod` and file `go.mod` exists.

- [ ] **Step 2: Create minimal main.go**

```go
package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("TaiSang-KB starting...")
	if err := http.ListenAndServe("127.0.0.1:8080", nil); err != nil {
		log.Fatal(err)
	}
}
```

- [ ] **Step 3: Create .gitignore**

```
# Binaries
/bin/
*.exe

# Env
.env
.env.local

# IDE
.idea/
.vscode/

# OS
.DS_Store
Thumbs.db

# Frontend
/frontend/node_modules/
/frontend/dist/

# Storage
/storage/

# Logs
*.log
```

- [ ] **Step 4: Create Makefile**

```makefile
.PHONY: dev build run test fmt

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

dev:
	go run ./cmd/server

test:
	go test ./... -count=1

fmt:
	go fmt ./...
```

- [ ] **Step 5: Verify build**

Run:
```bash
cd D:/Project/TaiSang-KB
go build ./cmd/server
```
Expected: no errors, `bin/server` not yet but build succeeds (binary goes to current dir unless `-o`). Verify no error output.

- [ ] **Step 6: Commit**

```bash
cd D:/Project/TaiSang-KB
git add go.mod cmd/ Makefile .gitignore
git commit -m "chore: init go module and project skeleton"
```

---

### Task 2: Add core dependencies

**Files:**
- Modify: `go.mod` (via go get)
- Modify: `cmd/server/main.go` (smoke test wiring, will be replaced later)

- [ ] **Step 1: Add dependencies**

Run:
```bash
cd D:/Project/TaiSang-KB
go get github.com/gin-gonic/gin@latest
go get gorm.io/gorm@latest
go get gorm.io/driver/postgres@latest
go get github.com/pgvector/pgvector-go@latest
go get go.uber.org/zap@latest
```
Expected: `go.mod` and `go.sum` updated with these deps.

- [ ] **Step 2: Tidy**

Run:
```bash
go mod tidy
```
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add gin gorm pgvector zap dependencies"
```

---

### Task 3: Config loader (env → struct)

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
package config

import (
	"os"
	"testing"
)

func TestLoad_FromEnv(t *testing.T) {
	os.Setenv("TSK_DB_DSN", "postgres://user:pass@localhost:5432/db?sslmode=disable")
	os.Setenv("TSK_LISTEN_ADDR", "127.0.0.1:8080")
	os.Setenv("TSK_STORAGE_DIR", "./storage")
	os.Setenv("TSK_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	os.Setenv("TSK_EMBEDDING_DIM", "1536")
	defer func() {
		os.Unsetenv("TSK_DB_DSN")
		os.Unsetenv("TSK_LISTEN_ADDR")
		os.Unsetenv("TSK_STORAGE_DIR")
		os.Unsetenv("TSK_ENCRYPTION_KEY")
		os.Unsetenv("TSK_EMBEDDING_DIM")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.DBDSN != "postgres://user:pass@localhost:5432/db?sslmode=disable" {
		t.Errorf("DBDSN mismatch: %s", cfg.DBDSN)
	}
	if cfg.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("ListenAddr mismatch: %s", cfg.ListenAddr)
	}
	if cfg.EmbeddingDim != 1536 {
		t.Errorf("EmbeddingDim mismatch: %d", cfg.EmbeddingDim)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	os.Unsetenv("TSK_DB_DSN")
	os.Unsetenv("TSK_ENCRYPTION_KEY")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required env, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/config/ -v
```
Expected: FAIL with "package internal/config not found" or build error (no config.go yet).

- [ ] **Step 3: Write minimal implementation**

```go
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DBDSN         string
	ListenAddr    string
	StorageDir    string
	EncryptionKey string
	EmbeddingDim  int
}

func Load() (*Config, error) {
	dsn := os.Getenv("TSK_DB_DSN")
	if dsn == "" {
		return nil, fmt.Errorf("TSK_DB_DSN is required")
	}
	key := os.Getenv("TSK_ENCRYPTION_KEY")
	if len(key) != 32 {
		return nil, fmt.Errorf("TSK_ENCRYPTION_KEY must be 32 chars (for AES-256-GCM)")
	}

	addr := os.Getenv("TSK_LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	storage := os.Getenv("TSK_STORAGE_DIR")
	if storage == "" {
		storage = "./storage"
	}
	dim := 1536
	if s := os.Getenv("TSK_EMBEDDING_DIM"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("TSK_EMBEDDING_DIM invalid: %w", err)
		}
		dim = v
	}

	return &Config{
		DBDSN:         dsn,
		ListenAddr:    addr,
		StorageDir:    storage,
		EncryptionKey: key,
		EmbeddingDim:  dim,
	}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/config/ -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): env-based config loader with required-field validation"
```

---

### Task 4: Structured logger (zap)

**Files:**
- Create: `internal/logger/logger.go`

- [ ] **Step 1: Create logger package**

```go
package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var L *zap.Logger

func Init() error {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	l, err := cfg.Build()
	if err != nil {
		return err
	}
	L = l
	return nil
}

func Sync() {
	if L != nil {
		_ = L.Sync()
	}
}
```

- [ ] **Step 2: Smoke test in main.go (temporary)**

Replace `cmd/server/main.go`:
```go
package main

import (
	"log"

	"github.com/zht475706171/TaiSang-KB/internal/config"
	"github.com/zht475706171/TaiSang-KB/internal/logger"
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
	logger.L.Info("config loaded",
		zap.String("addr", cfg.ListenAddr),
		zap.String("storage", cfg.StorageDir),
	)
}
```

Note: add `"go.uber.org/zap"` import for `zap.Error`/`zap.String`. The full import block:
```go
import (
	"log"

	"go.uber.org/zap"

	"github.com/zht475706171/TaiSang-KB/internal/config"
	"github.com/zht475706171/TaiSang-KB/internal/logger"
)
```

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./cmd/server
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/logger/ cmd/server/main.go
git commit -m "feat(logger): zap structured logger init"
```

---

### Task 5: GORM models

**Files:**
- Create: `internal/models/models.go`

- [ ] **Step 1: Create models package**

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// KnowledgeBase is a collection of documents.
type KnowledgeBase struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	Name           string         `gorm:"size:255;not null" json:"name"`
	Description    string         `gorm:"size:1024" json:"description"`
	EmbeddingModel string         `gorm:"size:128" json:"embedding_model"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// Document is an uploaded file in a KB.
type Document struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	KBID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"kb_id"`
	Title      string         `gorm:"size:512;not null" json:"title"`
	Source     string         `gorm:"size:32;not null;default:'upload'" json:"source"`
	FilePath   string         `gorm:"size:1024;not null" json:"file_path"`
	MimeType   string         `gorm:"size:128" json:"mime_type"`
	Status     string         `gorm:"size:32;not null;default:'pending';index" json:"status"`
	ErrorMsg   string         `gorm:"size:1024" json:"error_msg"`
	Meta       datatypes.JSON `gorm:"type:jsonb" json:"meta"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Chunk is a searchable text slice of a document.
type Chunk struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	DocID           uuid.UUID `gorm:"type:uuid;not null;index" json:"doc_id"`
	Content         string    `gorm:"type:text;not null" json:"content"`
	ParentContent   string    `gorm:"type:text" json:"parent_content"`
	ChunkIndex      int       `gorm:"not null" json:"chunk_index"`
	TokenCount      int       `json:"token_count"`
	EmbeddingRetry  bool      `gorm:"default:false" json:"embedding_retry"`
	CreatedAt       time.Time `json:"created_at"`
	// Embedding is set via raw SQL (pgvector type); not declared here to avoid driver complexity.
}

// ChatSession groups messages in one conversation.
type ChatSession struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Title     string    `gorm:"size:255" json:"title"`
	KBID      *uuid.UUID `gorm:"type:uuid;index" json:"kb_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatMessage is one user or assistant turn.
type ChatMessage struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	SessionID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"session_id"`
	Role       string         `gorm:"size:32;not null" json:"role"`
	Content    string         `gorm:"type:text;not null" json:"content"`
	Citations  datatypes.JSON `gorm:"type:jsonb" json:"citations"`
	CreatedAt  time.Time      `json:"created_at"`
}

// Setting is the single-row app settings table.
type Setting struct {
	ID              int    `gorm:"primaryKey" json:"id"`
	APIBaseURL      string `gorm:"size:512" json:"api_base_url"`
	APIKey          string `gorm:"size:1024" json:"api_key"` // encrypted
	ChatModel       string `gorm:"size:128" json:"chat_model"`
	EmbeddingModel  string `gorm:"size:128" json:"embedding_model"`
	RerankEnabled   bool   `json:"rerank_enabled"`
	TopK            int    `gorm:"default:4" json:"top_k"`
}

func (KnowledgeBase) TableName() string { return "knowledge_base" }
func (Document) TableName() string      { return "document" }
func (Chunk) TableName() string         { return "chunk" }
func (ChatSession) TableName() string   { return "chat_session" }
func (ChatMessage) TableName() string   { return "chat_message" }
func (Setting) TableName() string       { return "setting" }
```

- [ ] **Step 2: Add uuid dependency**

Run:
```bash
go get github.com/google/uuid@latest
go get gorm.io/datatypes@latest
```

- [ ] **Step 3: Verify build**

Run:
```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/models/ go.mod go.sum
git commit -m "feat(models): GORM entities for kb/document/chunk/chat/setting"
```

---

### Task 6: DB connection and AutoMigrate

**Files:**
- Create: `internal/database/db.go`
- Create: `internal/database/db_test.go`

- [ ] **Step 1: Write the failing test (integration with testcontainers)**

```go
package database

import (
	"testing"

	"github.com/zht475706171/TaiSang-KB/internal/models"
)

func TestConnectAndMigrate(t *testing.T) {
	// Skip in CI without docker; local run requires docker.
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dsn := "postgres://postgres:postgres@localhost:5433/taisang_test?sslmode=disable"
	// Expects a postgres+pgvector running on 5433. For CI use testcontainers; see Step 3 note.
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
	// settings default row
	var s models.Setting
	if err := db.First(&s, 1).Error; err != nil {
		t.Errorf("default setting row missing: %v", err)
	}
}
```

Note: this test skips if PG not available on 5433. To actually run it, start a throwaway container:
```bash
docker run -d --name tsk-pgtest -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=taisang_test -p 5433:5432 pgvector/pgvector:pg14
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/database/ -v
```
Expected: FAIL (no db.go).

- [ ] **Step 3: Write implementation**

```go
package database

import (
	"fmt"

	"github.com/pgvector/pgvector-go"
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
	// Ensure pgvector extension and register type.
	if err := db.Exec(`CREATE EXTENSION IF NOT EXISTS vector`).Error; err != nil {
		return fmt.Errorf("create extension vector: %w", err)
	}
	pgvector.RegisterVector(db)

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

	// Add embedding column (vector(1536)) to chunk via raw SQL.
	if !db.Migrator().HasColumn(&models.Chunk{}, "embedding") {
		if err := db.Exec(`ALTER TABLE chunk ADD COLUMN embedding vector(1536)`).Error; err != nil {
			return fmt.Errorf("add embedding column: %w", err)
		}
	}

	// HNSW index on embedding.
	if err := db.Exec(`CREATE INDEX IF NOT EXISTS chunk_embedding_hnsw
		ON chunk USING hnsw (embedding vector_cosine_ops)`).Error; err != nil {
		return fmt.Errorf("create hnsw index: %w", err)
	}

	// Default settings row (id=1) if not exists.
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
```

- [ ] **Step 4: Run test to verify it passes**

Start the test container:
```bash
docker run -d --name tsk-pgtest -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=taisang_test -p 5433:5432 pgvector/pgvector:pg14
```
Wait ~5s for PG to be ready, then:
```bash
go test ./internal/database/ -v
```
Expected: PASS (all tables exist, default setting row created).

Cleanup after:
```bash
docker rm -f tsk-pgtest
```

- [ ] **Step 5: Commit**

```bash
git add internal/database/
git commit -m "feat(db): postgres connection + AutoMigrate with pgvector HNSW index"
```

---

### Task 7: Docker compose for local dev

**Files:**
- Create: `docker-compose.yml`
- Create: `.env.example`

- [ ] **Step 1: Create docker-compose.yml**

```yaml
services:
  postgres:
    image: pgvector/pgvector:pg14
    container_name: tsk-postgres
    environment:
      POSTGRES_USER: tsk
      POSTGRES_PASSWORD: tsk
      POSTGRES_DB: taisang
    ports:
      - "127.0.0.1:5432:5432"
    volumes:
      - tsk_pg:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U tsk -d taisang"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  tsk_pg:
```

- [ ] **Step 2: Create .env.example**

```bash
# === Required ===
# Postgres DSN. Matches docker-compose.yml above by default.
TSK_DB_DSN=postgres://tsk:tsk@localhost:5432/taisang?sslmode=disable

# 32-char key for AES-256-GCM (used to encrypt API Key in settings table).
# Generate with: openssl rand -hex 16
TSK_ENCRYPTION_KEY=change_me_to_32_char_random_string

# === Optional ===
TSK_LISTEN_ADDR=127.0.0.1:8080
TSK_STORAGE_DIR=./storage
TSK_EMBEDDING_DIM=1536
```

- [ ] **Step 3: Verify compose file**

Run:
```bash
docker compose config
```
Expected: valid YAML, no errors.

- [ ] **Step 4: Commit**

```bash
git add docker-compose.yml .env.example
git commit -m "chore: docker compose for local postgres+pgvector"
```

---

### Task 8: Wire main.go with config + db + health endpoint

**Files:**
- Modify: `cmd/server/main.go`
- Create: `internal/router/router.go`
- Create: `internal/router/router_test.go`

- [ ] **Step 1: Write the failing test for health endpoint**

```go
package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	r := New(nil) // nil db for health (just returns ok)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Errorf("body = %s, want contains 'ok'", w.Body.String())
	}
}
```

Add missing import `"strings"` at top.

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/router/ -v
```
Expected: FAIL (no router.go).

- [ ] **Step 3: Write router implementation**

```go
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func New(db *gorm.DB) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	{
		api.GET("/health", healthHandler)
	}

	return r
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/router/ -v
```
Expected: PASS.

- [ ] **Step 5: Wire main.go**

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
	logger.L.Info("db migrated")

	r := router.New(db)
	logger.L.Info("listening", zap.String("addr", cfg.ListenAddr))
	if err := r.Run(cfg.ListenAddr); err != nil {
		logger.L.Fatal("server stopped", zap.Error(err))
	}
}
```

- [ ] **Step 6: Verify build**

Run:
```bash
go build ./cmd/server
```
Expected: no errors.

- [ ] **Step 7: End-to-end smoke (manual)**

Start postgres:
```bash
docker compose up -d postgres
```
Wait for healthy, then in another terminal set env and run server:
```bash
cp .env.example .env
# Edit .env: set TSK_ENCRYPTION_KEY to a 32-char string
export $(grep -v '^#' .env | xargs)
go run ./cmd/server
```
Expected: log "db migrated" + "listening addr=127.0.0.1:8080". In another terminal:
```bash
curl http://127.0.0.1:8080/api/health
```
Expected: `{"status":"ok"}`.

Stop with Ctrl+C. `docker compose down` (keep volume) or `docker compose down -v` (wipe).

- [ ] **Step 8: Commit**

```bash
git add cmd/server/main.go internal/router/
git commit -m "feat(router): wire main.go with config+db+health endpoint"
```

---

### Task 9: Storage package (local file bucket)

**Files:**
- Create: `internal/infrastructure/storage/storage.go`
- Create: `internal/infrastructure/storage/storage_test.go`

- [ ] **Step 1: Write the failing test**

```go
package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveFile_RenamesAndPersists(t *testing.T) {
	tmp := t.TempDir()
	s := New(tmp)

	body := []byte("hello world")
	savedPath, err := s.Save("report.pdf", "application/pdf", body)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	full := filepath.Join(tmp, savedPath)
	got, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("content = %q", string(got))
	}

	// filename should not contain the original name (UUID rename)
	if filepath.Base(savedPath) == "report.pdf" {
		t.Error("expected UUID-renamed file, got original name")
	}
}

func TestSaveFile_SanitizesTrickyNames(t *testing.T) {
	tmp := t.TempDir()
	s := New(tmp)
	if _, err := s.Save("../../etc/passwd", "text/plain", []byte("x")); err != nil {
		t.Errorf("expected sanitized save, got: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/infrastructure/storage/ -v
```
Expected: FAIL (no storage.go).

- [ ] **Step 3: Write implementation**

```go
package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type Storage struct {
	root string
}

func New(root string) *Storage {
	return &Storage{root: root}
}

// Save persists body under a fresh UUID-derived filename. Returns the path
// relative to root (e.g. "ab/cd/abcd....pdf"). Original name is sanitized away
// to prevent path traversal; caller stores original name in document meta.
func (s *Storage) Save(originalName, mimeType string, body []byte) (string, error) {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return "", fmt.Errorf("mkdir storage: %w", err)
	}
	id := uuid.NewString()
	ext := sanitizeExt(filepath.Ext(originalName))
	// Two-level sharding to avoid 100k files in one dir.
	prefix := id[:2]
	subdir := id[2:4]
	rel := filepath.Join(prefix, subdir, id+ext)
	full := filepath.Join(s.root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, body, 0o644); err != nil {
		return "", err
	}
	return rel, nil
}

func (s *Storage) FullPath(rel string) string {
	// Prevent traversal: clean and ensure within root.
	cleaned := filepath.Clean(rel)
	if strings.HasPrefix(cleaned, "..") {
		return ""
	}
	return filepath.Join(s.root, cleaned)
}

func sanitizeExt(ext string) string {
	ext = strings.ToLower(ext)
	if ext == "" || len(ext) > 16 {
		return ""
	}
	for _, r := range ext {
		if !(r == '.' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return ""
		}
	}
	return ext
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/infrastructure/storage/ -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/storage/
git commit -m "feat(storage): local file bucket with UUID rename and traversal guard"
```

---

### Task 10: Crypto helper (AES-256-GCM for API Key)

**Files:**
- Create: `internal/infrastructure/crypto/crypto.go`
- Create: `internal/infrastructure/crypto/crypto_test.go`

- [ ] **Step 1: Write the failing test**

```go
package crypto

import (
	"testing"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef" // 32 chars
	c, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	plain := "sk-abcdef123456"
	enc, err := c.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if enc == plain {
		t.Error("ciphertext == plaintext")
	}
	dec, err := c.Decrypt(enc)
	if err != nil {
		t.Fatal(err)
	}
	if dec != plain {
		t.Errorf("got %q want %q", dec, plain)
	}
}

func TestMaskAPIKey(t *testing.T) {
	got := Mask("sk-abcdef123456")
	if got != "****3456" {
		t.Errorf("got %q", got)
	}
	if Mask("short") != "****" {
		t.Errorf("short key mask = %q", Mask("short"))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/infrastructure/crypto/ -v
```
Expected: FAIL.

- [ ] **Step 3: Write implementation**

```go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

type Crypto struct {
	key []byte
}

func New(key string) (*Crypto, error) {
	if len(key) != 32 {
		return nil, errors.New("key must be 32 bytes")
	}
	return &Crypto{key: []byte(key)}, nil
}

func (c *Crypto) Encrypt(plain string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

func (c *Crypto) Decrypt(enc string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// Mask returns ****last4 for display, or **** if too short.
func Mask(key string) string {
	if len(key) < 4 {
		return "****"
	}
	return "****" + key[len(key)-4:]
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/infrastructure/crypto/ -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/crypto/
git commit -m "feat(crypto): AES-256-GCM encrypt/decrypt + API key masking"
```

---

### Task 11: Self-review and checkpoint

- [ ] **Step 1: Run all tests**

```bash
go test ./... -count=1 -short
```
Expected: all unit tests PASS; integration tests skip.

- [ ] **Step 2: Run vet**

```bash
go vet ./...
```
Expected: no issues.

- [ ] **Step 3: Verify build**

```bash
go build ./...
```
Expected: no errors.

- [ ] **Step 4: Verify end-to-end**

```bash
docker compose up -d postgres
cp .env.example .env
# Edit .env: set TSK_ENCRYPTION_KEY=change_me_to_32_char_random_string (must be exactly 32 chars)
export $(grep -v '^#' .env | xargs)
go run ./cmd/server
# In another terminal:
curl http://127.0.0.1:8080/api/health
# Expected: {"status":"ok"}
```

- [ ] **Step 5: Commit checkpoint**

```bash
git add -A
git commit --allow-empty -m "checkpoint: Part 1 foundation complete (config/db/models/router/storage/crypto)"
```

---

## End of Part 1

**What's done:**
- Project skeleton (Go module, Makefile, .gitignore)
- Config loader with env validation
- zap structured logger
- GORM models for all 6 tables
- DB connection + AutoMigrate with pgvector HNSW index
- docker compose for local postgres+pgvector
- Health endpoint + main.go wiring
- Local file storage with UUID rename + traversal guard
- AES-256-GCM crypto for API key

**Next parts (separate plan files):**
- Part 2: Parser (MD/TXT/HTML/PDF/DOCX/XLSX/PPTX) + Chunker (父子分块)
- Part 3: LLM client + Embedding client + Reranker
- Part 4: KB/Document/Chat/Setting services + handlers + async parse worker
- Part 5: Frontend (Vue 3 + TDesign, three pages)
- Part 6: docker compose full stack + Dockerfile + README