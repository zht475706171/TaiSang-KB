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
	".docx": true, ".xlsx": true, // pptx v1 不支持
}

type Service struct {
	docRepo *postgres.DocumentRepository
	storage *storage.Storage
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