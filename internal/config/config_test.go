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