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