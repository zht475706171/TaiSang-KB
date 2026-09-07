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