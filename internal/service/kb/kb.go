package kb

import (
	"errors"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

type Service struct {
	kbRepo  *postgres.KBRepository
	docRepo *postgres.DocumentRepository
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