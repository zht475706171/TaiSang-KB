package kb

import (
	"testing"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

func TestKB_CRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)
	svc := NewService(postgres.NewKBRepository(db), postgres.NewDocumentRepository(db))

	// Create
	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "Test KB", Description: "d"}
	if err := svc.Create(kb); err != nil {
		t.Fatal(err)
	}
	// Create validation: empty name
	if err := svc.Create(&models.KnowledgeBase{ID: uuid.New(), Name: ""}); err == nil {
		t.Error("expected error for empty name")
	}

	// Get
	got, err := svc.Get(kb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != "Test KB" {
		t.Fatalf("get failed: %v", got)
	}

	// List
	list, err := svc.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Errorf("list len = %d", len(list))
	}

	// Update
	kb.Description = "updated"
	if err := svc.Update(kb); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Get(kb.ID)
	if got.Description != "updated" {
		t.Errorf("desc = %s", got.Description)
	}
	// Update validation
	kb.Name = ""
	if err := svc.Update(kb); err == nil {
		t.Error("expected error for empty name on update")
	}

	// Delete
	if err := svc.Delete(kb.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Get(kb.ID)
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestKB_DeleteCascadesDocuments(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)
	kbSvc := NewService(postgres.NewKBRepository(db), postgres.NewDocumentRepository(db))
	docRepo := postgres.NewDocumentRepository(db)

	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "kb"}
	kbSvc.Create(kb)
	doc := &models.Document{ID: uuid.New(), KBID: kb.ID, Title: "d", FilePath: "x", Status: "pending"}
	docRepo.Create(doc)

	if err := kbSvc.Delete(kb.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := docRepo.Get(doc.ID)
	if got != nil {
		t.Error("document should be cascade-deleted with kb")
	}
}