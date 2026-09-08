package worker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/llmclient"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/storage"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"github.com/zht475706171/TaiSang-KB/internal/service/chunker"
)

// makeVec returns 1536-dim vector with first 3 set.
func makeVec(a, b, c float32) []float32 {
	v := make([]float32, 1536)
	v[0], v[1], v[2] = a, b, c
	return v
}

func TestParseWorker_PipelineEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)

	embedSrv := startFakeEmbed(t)
	defer embedSrv.Close()
	embedC := llmclient.NewEmbeddingClient(embedSrv.URL, "sk-test", "test-model", 64)

	tmp := t.TempDir()
	st := storage.New(tmp)

	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "kb"}
	if err := db.Create(kb).Error; err != nil {
		t.Fatal(err)
	}

	body := []byte("# Hello\n\nWorld content here.")
	rel, err := st.Save("input.md", "text/markdown", body)
	if err != nil {
		t.Fatal(err)
	}
	doc := &models.Document{
		ID: uuid.New(), KBID: kb.ID, Title: "input.md",
		FilePath: rel, MimeType: "text/markdown", Status: "pending",
	}
	if err := db.Create(doc).Error; err != nil {
		t.Fatal(err)
	}

	w := NewParseWorker(db, st, embedC, chunker.Split)
	w.ProcessOnce(doc.ID)

	var got models.Document
	if err := db.First(&got, doc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != "done" {
		t.Fatalf("status = %s, err = %s", got.Status, got.ErrorMsg)
	}
	var n int64
	db.Model(&models.Chunk{}).Where("doc_id = ?", doc.ID).Count(&n)
	if n == 0 {
		t.Errorf("no chunks created")
	}
}

func TestParseWorker_FailedDocMarksFailed(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)

	// embed server that returns 500 to trigger failure
	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer embedSrv.Close()
	embedC := llmclient.NewEmbeddingClient(embedSrv.URL, "sk-test", "test-model", 64)

	tmp := t.TempDir()
	st := storage.New(tmp)

	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "kb"}
	db.Create(kb)

	body := []byte("# Hello\n\nWorld.")
	rel, _ := st.Save("input.md", "text/markdown", body)
	doc := &models.Document{
		ID: uuid.New(), KBID: kb.ID, Title: "input.md",
		FilePath: rel, MimeType: "text/markdown", Status: "pending",
	}
	db.Create(doc)

	w := NewParseWorker(db, st, embedC, chunker.Split)
	w.ProcessOnce(doc.ID)

	var got models.Document
	db.First(&got, doc.ID)
	if got.Status != "failed" {
		t.Errorf("status = %s, want failed", got.Status)
	}
	if got.ErrorMsg == "" {
		t.Error("expected error_msg to be set")
	}
}

// startFakeEmbed returns 1536-dim vectors (first 3 set, rest 0).
func startFakeEmbed(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llmclient.EmbedRequest
		json.NewDecoder(r.Body).Decode(&req)
		resp := llmclient.EmbedResponse{Data: make([]llmclient.EmbedItem, len(req.Input))}
		for i := range req.Input {
			resp.Data[i].Embedding = makeVec(0.1, 0.2, 0.3)
			resp.Data[i].Index = i
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}