package postgres

import (
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"gorm.io/gorm"
)

type SettingRepository struct{ db *gorm.DB }

func NewSettingRepository(db *gorm.DB) *SettingRepository { return &SettingRepository{db: db} }

func (r *SettingRepository) Get() (*models.Setting, error) {
	var s models.Setting
	err := r.db.First(&s, 1).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &s, err
}

func (r *SettingRepository) Update(s *models.Setting) error {
	return r.db.Save(s).Error
}