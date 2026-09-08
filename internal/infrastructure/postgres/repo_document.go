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

func (r *DocumentRepository) GetDB() *gorm.DB { return r.db }