package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"github.com/zht475706171/TaiSang-KB/internal/service/kb"
)

type KBHandler struct {
	svc *kb.Service
}

func NewKBHandler(svc *kb.Service) *KBHandler { return &KBHandler{svc: svc} }

func (h *KBHandler) Create(c *gin.Context) {
	var body struct {
		Name           string `json:"name" binding:"required"`
		Description    string `json:"description"`
		EmbeddingModel string `json:"embedding_model"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	k := &models.KnowledgeBase{
		ID: uuid.New(), Name: body.Name, Description: body.Description, EmbeddingModel: body.EmbeddingModel,
	}
	if err := h.svc.Create(k); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, k)
}

func (h *KBHandler) List(c *gin.Context) {
	kbs, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": kbs})
}

func (h *KBHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	k, err := h.svc.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if k == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, k)
}

func (h *KBHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body struct {
		Name           string `json:"name"`
		Description    string `json:"description"`
		EmbeddingModel string `json:"embedding_model"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	k, err := h.svc.Get(id)
	if err != nil || k == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	k.Name = body.Name
	k.Description = body.Description
	k.EmbeddingModel = body.EmbeddingModel
	if err := h.svc.Update(k); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, k)
}

func (h *KBHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.svc.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}