package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/llmclient"
	"github.com/zht475706171/TaiSang-KB/internal/infrastructure/postgres"
	"github.com/zht475706171/TaiSang-KB/internal/models"
)

// SettingProvider is the subset of setting.Service the chat service needs.
type SettingProvider interface {
	GetDecrypted() (*models.Setting, error)
}

// Citation is a retrieved chunk referenced by the assistant answer.
type Citation struct {
	ChunkID  uuid.UUID `json:"chunk_id"`
	DocID    uuid.UUID `json:"doc_id"`
	DocTitle string    `json:"doc_title"`
	Snippet  string    `json:"content_snippet"`
	Score    float64   `json:"score"`
}

// Service implements RAG chat: embed question → retrieve → (rerank) → stream LLM answer → citations.
type Service struct {
	chatRepo  *postgres.ChatRepository
	chunkRepo *postgres.ChunkRepository
	docRepo   *postgres.DocumentRepository
	setting   SettingProvider
	embed     *llmclient.EmbeddingClient
	chat      *llmclient.ChatClient
	rerank    *llmclient.RerankClient
}

// NewService constructs a chat Service. docRepo is used to resolve citation titles.
func NewService(
	chatRepo *postgres.ChatRepository,
	chunkRepo *postgres.ChunkRepository,
	docRepo *postgres.DocumentRepository,
	setting SettingProvider,
	embed *llmclient.EmbeddingClient,
	chat *llmclient.ChatClient,
	rerank *llmclient.RerankClient,
) *Service {
	return &Service{chatRepo, chunkRepo, docRepo, setting, embed, chat, rerank}
}

// Chat runs RAG for a single question. onDelta is called for each streamed token.
// If sessionID is uuid.Nil, a new session is created. Returns citations for the answer.
func (s *Service) Chat(kbID uuid.UUID, question string, sessionID uuid.UUID, onDelta func(string) error) ([]Citation, error) {
	if strings.TrimSpace(question) == "" {
		return nil, errors.New("empty question")
	}
	setting, err := s.setting.GetDecrypted()
	if err != nil {
		return nil, fmt.Errorf("setting: %w", err)
	}
	if setting == nil || setting.APIKey == "" || setting.ChatModel == "" {
		return nil, errors.New("api_key or chat_model not configured")
	}

	// 1. Embed question
	qvecs, err := s.embed.EmbedBatch([]string{question})
	if err != nil {
		return nil, fmt.Errorf("embed question: %w", err)
	}
	if len(qvecs) == 0 {
		return nil, errors.New("empty embedding")
	}

	// 2. Retrieve
	hits, err := s.chunkRepo.SearchByVector(qvecs[0], kbID, 20)
	if err != nil {
		return nil, fmt.Errorf("retrieve: %w", err)
	}
	if len(hits) == 0 {
		fallback := "知识库中未找到相关内容，请尝试换种问法或上传更多文档。"
		_ = onDelta(fallback)
		return nil, s.persistTurn(sessionID, kbID, question, fallback, nil)
	}

	// 3. Rerank (optional)
	topK := setting.TopK
	if topK <= 0 {
		topK = 4
	}
	if s.rerank != nil && setting.RerankEnabled {
		docs := make([]string, len(hits))
		for i, h := range hits {
			docs[i] = h.Content
		}
		rr, err := s.rerank.Rerank(question, docs)
		if err == nil && len(rr) > 0 && len(rr) <= len(hits) {
			reordered := make([]postgres.ChunkSearchResult, len(rr))
			for i, item := range rr {
				if item.Index >= 0 && item.Index < len(hits) {
					reordered[i] = hits[item.Index]
					reordered[i].Score = item.RelevanceScore
				}
			}
			hits = reordered
		}
	}
	if len(hits) > topK {
		hits = hits[:topK]
	}

	// 4. Build citations + context
	citations := make([]Citation, len(hits))
	var contextBuf strings.Builder
	for i, h := range hits {
		title := ""
		if d, err := s.docRepo.Get(h.DocID); err == nil && d != nil {
			title = d.Title
		}
		citations[i] = Citation{
			ChunkID: h.ID, DocID: h.DocID, DocTitle: title,
			Snippet: truncate(h.Content, 200), Score: h.Score,
		}
		contextBuf.WriteString(h.ParentContent)
		contextBuf.WriteString("\n\n")
	}

	// 5. Build messages
	system := "你是知识库问答助手。根据以下检索到的内容回答用户问题。如果检索内容不足以回答，请如实说明。"
	retrieved := fmt.Sprintf("检索到的内容：\n%s", contextBuf.String())
	msgs := []llmclient.ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: retrieved + "\n\n用户问题：" + question},
	}

	// 6. Stream
	var answer strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_ = ctx
	streamErr := s.chat.StreamChat(msgs, func(delta string) error {
		answer.WriteString(delta)
		return onDelta(delta)
	})
	if streamErr != nil {
		_ = s.persistTurn(sessionID, kbID, question, answer.String(), citations)
		return citations, streamErr
	}

	// 7. Persist
	if err := s.persistTurn(sessionID, kbID, question, answer.String(), citations); err != nil {
		return citations, err
	}
	return citations, nil
}

// persistTurn creates a session (if sessionID is Nil) and stores user + assistant messages.
func (s *Service) persistTurn(sessionID uuid.UUID, kbID uuid.UUID, question, answer string, citations []Citation) error {
	if sessionID == uuid.Nil {
		sessionID = uuid.New()
		session := &models.ChatSession{ID: sessionID, KBID: &kbID, Title: truncate(question, 40)}
		if err := s.chatRepo.CreateSession(session); err != nil {
			return err
		}
	}
	if err := s.chatRepo.AddMessage(&models.ChatMessage{
		ID: uuid.New(), SessionID: sessionID, Role: "user", Content: question,
	}); err != nil {
		return err
	}
	citJSON, _ := json.Marshal(citations)
	return s.chatRepo.AddMessage(&models.ChatMessage{
		ID: uuid.New(), SessionID: sessionID, Role: "assistant", Content: answer, Citations: citJSON,
	})
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}