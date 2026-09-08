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
	for i, j := 0, len(ms)-1; i < j; i, j = i+1, j-1 {
		ms[i], ms[j] = ms[j], ms[i]
	}
	return ms, nil
}

func (r *ChatRepository) GetDB() *gorm.DB { return r.db }