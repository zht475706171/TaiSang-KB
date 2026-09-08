package setting

import (
	"errors"
	"strings"

	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/crypto"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"gorm.io/gorm"
)

// SettingRepository is the storage interface required by the setting service.
type SettingRepository interface {
	Get() (*models.Setting, error)
	Update(s *models.Setting) error
}

type Service struct {
	repo SettingRepository
	c    *crypto.Crypto
}

func NewService(repo SettingRepository, c *crypto.Crypto) *Service {
	return &Service{repo: repo, c: c}
}

// GetMasked returns the current setting with APIKey masked (****last4).
// If no setting row exists, returns a default setting (TopK=4, empty strings).
func (s *Service) GetMasked() (*models.Setting, error) {
	st, err := s.repo.Get()
	if err != nil {
		return nil, err
	}
	if st == nil {
		return defaultSetting(), nil
	}
	if st.APIKey == "" {
		return st, nil
	}
	plain, err := s.c.Decrypt(st.APIKey)
	if err != nil {
		return nil, err
	}
	st.APIKey = crypto.Mask(plain)
	return st, nil
}

// GetDecrypted returns the current setting with APIKey decrypted (for internal use
// by chat/embedding clients). Empty key stays empty.
func (s *Service) GetDecrypted() (*models.Setting, error) {
	st, err := s.repo.Get()
	if err != nil {
		return nil, err
	}
	if st == nil {
		return defaultSetting(), nil
	}
	if st.APIKey == "" {
		return st, nil
	}
	plain, err := s.c.Decrypt(st.APIKey)
	if err != nil {
		return nil, err
	}
	st.APIKey = plain
	return st, nil
}

// Update persists the setting. APIKey is encrypted before save.
// If the incoming APIKey is a masked value (**** prefix) or empty, the existing
// ciphertext is preserved so the frontend can submit the masked key without
// clobbering the real secret.
func (s *Service) Update(in *models.Setting) error {
	if in == nil {
		return errors.New("nil setting")
	}
	in.ID = 1

	key := strings.TrimSpace(in.APIKey)
	if key == "" || strings.HasPrefix(key, "****") {
		// preserve existing ciphertext
		cur, err := s.repo.Get()
		if err != nil {
			return err
		}
		if cur != nil && cur.APIKey != "" {
			in.APIKey = cur.APIKey
		} else {
			in.APIKey = ""
		}
	} else {
		enc, err := s.c.Encrypt(key)
		if err != nil {
			return err
		}
		in.APIKey = enc
	}
	return s.repo.Update(in)
}

func defaultSetting() *models.Setting {
	return &models.Setting{
		ID:    1,
		TopK:  4,
	}
}

// Compile-time guard to ensure gorm.ErrRecordNotFound is referenced (kept for
// future repository implementations that may switch on not-found).
var _ = gorm.ErrRecordNotFound