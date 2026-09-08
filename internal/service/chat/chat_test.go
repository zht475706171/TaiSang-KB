package chat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/llmclient"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

func makeVec(a, b, c float32) []float32 {
	v := make([]float32, 1536)
	v[0], v[1], v[2] = a, b, c
	return v
}

func TestChat_RAGStream(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)

	kb := &models.KnowledgeBase{ID: uuid.New(), Name: "kb"}
	if err := db.Create(kb).Error; err != nil {
		t.Fatal(err)
	}
	doc := &models.Document{ID: uuid.New(), KBID: kb.ID, Title: "earth.md", FilePath: "x", Status: "done"}
	if err := db.Create(doc).Error; err != nil {
		t.Fatal(err)
	}
	chunkRepo := postgres.NewChunkRepository(db)
	if err := chunkRepo.InsertBatchWithEmbedding(
		[]models.Chunk{{ID: uuid.New(), DocID: doc.ID, Content: "地球是圆的。", ParentContent: "地球是圆的。", ChunkIndex: 0, TokenCount: 5}},
		[][]float32{makeVec(0.1, 0.2, 0.3)},
	); err != nil {
		t.Fatal(err)
	}

	// Fake chat server streams "Answer: 地球是圆的"
	chatSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f := w.(http.Flusher)
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Answer: \"}}]}\n\n")
		f.Flush()
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"地球是圆的\"}}]}\n\n")
		f.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		f.Flush()
	}))
	defer chatSrv.Close()

	embedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer embedSrv.Close()

	settingSvc := newMockSetting("https://nope", "sk-test", "gpt-4o-mini", "test-model", false, 4)
	svc := NewService(
		postgres.NewChatRepository(db),
		postgres.NewChunkRepository(db),
		postgres.NewDocumentRepository(db),
		settingSvc,
		llmclient.NewEmbeddingClient(embedSrv.URL, "sk-test", "test-model", 64),
		llmclient.NewChatClient(chatSrv.URL, "sk-test", "gpt-4o-mini"),
		nil, // no rerank
	)

	var sb strings.Builder
	citations, err := svc.Chat(kb.ID, "地球什么形状？", uuid.Nil, func(delta string) error {
		sb.WriteString(delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "地球是圆的") {
		t.Errorf("answer = %q", sb.String())
	}
	if len(citations) == 0 {
		t.Errorf("expected citations")
	}
	if citations[0].DocTitle != "earth.md" {
		t.Errorf("citation doc title = %q, want 'earth.md'", citations[0].DocTitle)
	}

	// Verify session + messages persisted
	var sessionCount int64
	db.Model(&models.ChatSession{}).Count(&sessionCount)
	if sessionCount != 1 {
		t.Errorf("session count = %d, want 1", sessionCount)
	}
	var msgCount int64
	db.Model(&models.ChatMessage{}).Count(&msgCount)
	if msgCount != 2 { // user + assistant
		t.Errorf("msg count = %d, want 2", msgCount)
	}
}

func TestChat_EmptyQuestionReturnsError(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	db := postgres.OpenTestDB(t)
	svc := NewService(
		postgres.NewChatRepository(db),
		postgres.NewChunkRepository(db),
		postgres.NewDocumentRepository(db),
		newMockSetting("https://nope", "sk-test", "gpt-4o-mini", "test-model", false, 4),
		nil, nil, nil,
	)
	_, err := svc.Chat(uuid.New(), "   ", uuid.Nil, func(string) error { return nil })
	if err == nil {
		t.Error("expected error for empty question")
	}
}

// helpers
type mockSetting struct {
	base, key, chat, embed string
	rerank                 bool
	topK                   int
}

func newMockSetting(base, key, chat, embed string, rerank bool, topK int) *mockSetting {
	return &mockSetting{base, key, chat, embed, rerank, topK}
}
func (m *mockSetting) GetDecrypted() (*models.Setting, error) {
	return &models.Setting{ID: 1, APIBaseURL: m.base, APIKey: m.key, ChatModel: m.chat, EmbeddingModel: m.embed, RerankEnabled: m.rerank, TopK: m.topK}, nil
}