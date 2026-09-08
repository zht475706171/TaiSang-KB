package postgres

import (
	"testing"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

func TestKBRepository_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := setupTestDB(t)
	repo := NewKBRepository(db)

	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "Test KB", Description: "desc"}
	if err := repo.Create(kb); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(kb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != "Test KB" {
		t.Fatalf("get failed: %v", got)
	}
	list, err := repo.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("list len = %d", len(list))
	}
	kb.Description = "updated"
	if err := repo.Update(kb); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.Get(kb.ID)
	if got.Description != "updated" {
		t.Errorf("desc = %s", got.Description)
	}
	if err := repo.Delete(kb.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.Get(kb.ID)
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestDocumentRepository_ByKB(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := setupTestDB(t)
	kbRepo := NewKBRepository(db)
	docRepo := NewDocumentRepository(db)

	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "kb"}
	kbRepo.Create(kb)

	d1 := &models.Document{ID: uuid.New(), KBID: kb.ID, Title: "d1", FilePath: "a.md", Status: "pending"}
	d2 := &models.Document{ID: uuid.New(), KBID: kb.ID, Title: "d2", FilePath: "b.md", Status: "done"}
	docRepo.Create(d1)
	docRepo.Create(d2)

	list, err := docRepo.ListByKB(kb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Errorf("list len = %d", len(list))
	}
	if err := docRepo.UpdateStatus(d1.ID, "done", ""); err != nil {
		t.Fatal(err)
	}
	got, _ := docRepo.Get(d1.ID)
	if got.Status != "done" {
		t.Errorf("status = %s", got.Status)
	}
	pending, err := docRepo.ListByStatus("done")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Errorf("status list len = %d", len(pending))
	}
	if err := docRepo.Delete(d1.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = docRepo.ListByKB(kb.ID)
	if len(list) != 1 {
		t.Errorf("after delete len = %d", len(list))
	}
}

func TestChatRepository_SessionMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := setupTestDB(t)
	repo := NewChatRepository(db)

	s := &models.ChatSession{ID: uuid.New(), Title: "sess"}
	if err := repo.CreateSession(s); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetSession(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Title != "sess" {
		t.Fatalf("get session failed: %v", got)
	}
	if err := repo.UpdateSessionTitle(s.ID, "new title"); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetSession(s.ID)
	if got.Title != "new title" {
		t.Errorf("title = %s", got.Title)
	}
	list, err := repo.ListSessions(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("list len = %d", len(list))
	}
	// Add messages
	m1 := &models.ChatMessage{ID: uuid.New(), SessionID: s.ID, Role: "user", Content: "hi"}
	m2 := &models.ChatMessage{ID: uuid.New(), SessionID: s.ID, Role: "assistant", Content: "hello"}
	repo.AddMessage(m1)
	repo.AddMessage(m2)
	msgs, err := repo.ListMessages(s.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Errorf("msgs len = %d", len(msgs))
	}
	if msgs[0].Content != "hi" {
		t.Errorf("first msg = %s", msgs[0].Content)
	}
	recent, err := repo.RecentMessages(s.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 || recent[0].Content != "hello" {
		t.Errorf("recent = %v", recent)
	}
	if err := repo.DeleteSession(s.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.GetSession(s.ID)
	if got != nil {
		t.Error("session should be deleted")
	}
}

func TestSettingRepository_GetUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := setupTestDB(t)
	repo := NewSettingRepository(db)

	// setupTestDB truncates setting, but AutoMigrate doesn't reseed. Manually seed.
	s := &models.Setting{ID: 1, APIBaseURL: "https://x", ChatModel: "gpt", TopK: 4}
	if err := repo.Update(s); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.APIBaseURL != "https://x" {
		t.Fatalf("get failed: %v", got)
	}
	got.ChatModel = "gpt-4o"
	if err := repo.Update(got); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.Get()
	if got.ChatModel != "gpt-4o" {
		t.Errorf("chat model = %s", got.ChatModel)
	}
}