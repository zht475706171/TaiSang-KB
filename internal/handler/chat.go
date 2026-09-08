package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/service/chat"
)

type ChatHandler struct {
	svc *chat.Service
}

func NewChatHandler(svc *chat.Service) *ChatHandler { return &ChatHandler{svc: svc} }

func (h *ChatHandler) Chat(c *gin.Context) {
	var body struct {
		KBID      string `json:"kb_id" binding:"required"`
		Question  string `json:"question" binding:"required"`
		SessionID string `json:"session_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	kbID, err := uuid.Parse(body.KBID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid kb_id"})
		return
	}
	sessionID := uuid.Nil
	if body.SessionID != "" {
		sessionID, _ = uuid.Parse(body.SessionID)
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	citations, err := h.svc.Chat(kbID, body.Question, sessionID, func(delta string) error {
		data, _ := json.Marshal(map[string]string{"delta": delta})
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
		return nil
	})
	if err != nil {
		errData, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(c.Writer, "data: %s\n\n", errData)
		flusher.Flush()
		return
	}
	citData, _ := json.Marshal(map[string]any{"citations": citations})
	fmt.Fprintf(c.Writer, "data: %s\n\n", citData)
	flusher.Flush()
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}

// ListSessions is a stub for v1 (frontend tracks session_id from chat response).
func (h *ChatHandler) ListSessions(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (h *ChatHandler) GetMessages(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (h *ChatHandler) DeleteSession(c *gin.Context) {
	c.JSON(http.StatusNoContent, nil)
}