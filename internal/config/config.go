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