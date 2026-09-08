package setting

import (
	"testing"

	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/crypto"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

func TestSetting_GetMasksAPIKey(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)
	c, _ := crypto.New("0123456789abcdef0123456789abcdef")
	svc := NewService(postgres.NewSettingRepository(db), c)

	// Seed: Update 会加密 APIKey
	if err := svc.Update(&models.Setting{
		ID: 1, APIBaseURL: "https://api.x.com", APIKey: "sk-secret123456",
		ChatModel: "gpt-4o-mini", EmbeddingModel: "text-embedding-3-small",
		RerankEnabled: false, TopK: 4,
	}); err != nil {
		t.Fatal(err)
	}

	got, err := svc.GetMasked()
	if err != nil {
		t.Fatal(err)
	}
	if got.APIKey != "****3456" {
		t.Errorf("masked key = %q, want '****3456'", got.APIKey)
	}
	if got.APIBaseURL != "https://api.x.com" {
		t.Errorf("base url = %s", got.APIBaseURL)
	}

	plain, err := svc.GetDecrypted()
	if err != nil {
		t.Fatal(err)
	}
	if plain.APIKey != "sk-secret123456" {
		t.Errorf("decrypted = %q", plain.APIKey)
	}
}

func TestSetting_UpdatePreservesKeyWhenMasked(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)
	c, _ := crypto.New("0123456789abcdef0123456789abcdef")
	svc := NewService(postgres.NewSettingRepository(db), c)

	// 初始保存
	svc.Update(&models.Setting{
		ID: 1, APIBaseURL: "https://api.x.com", APIKey: "sk-secret123456",
		ChatModel: "gpt-4o-mini", TopK: 4,
	})

	// 前端回传 masked key（用户没改 key），应该保留原密文
	masked, _ := svc.GetMasked()
	masked.APIBaseURL = "https://api.y.com" // 改了 base url
	if err := svc.Update(masked); err != nil {
		t.Fatal(err)
	}

	plain, _ := svc.GetDecrypted()
	if plain.APIKey != "sk-secret123456" {
		t.Errorf("key changed after masked update: %q", plain.APIKey)
	}
	if plain.APIBaseURL != "https://api.y.com" {
		t.Errorf("base url not updated: %s", plain.APIBaseURL)
	}
}

func TestSetting_GetMaskedEmptyWhenNoKey(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)
	c, _ := crypto.New("0123456789abcdef0123456789abcdef")
	svc := NewService(postgres.NewSettingRepository(db), c)

	// 没设置过任何 setting，repo.Get() 返回 nil
	got, err := svc.GetMasked()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected non-nil default setting")
	}
	if got.APIKey != "" {
		t.Errorf("empty key = %q", got.APIKey)
	}
	if got.TopK != 4 {
		t.Errorf("default topk = %d, want 4", got.TopK)
	}
}