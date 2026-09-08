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
		if err := tx.Exec(`DELETE FROM chunk WHERE doc_id IN (SELECT id FROM document WHERE kb_id = ?)`, id).Error; err != nil {
			return err
		}
		if err := tx.Where("kb_id = ?", id).Delete(&models.Document{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.KnowledgeBase{}, "id = ?", id).Error
	})
}