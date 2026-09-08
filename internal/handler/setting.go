package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zht475706171/TaiSang-KB/internal/models"
	"github.com/zht475706171/TaiSang-KB/internal/service/setting"
)

type SettingHandler struct {
	svc *setting.Service
}

func NewSettingHandler(svc *setting.Service) *SettingHandler { return &SettingHandler{svc: svc} }

func (h *SettingHandler) Get(c *gin.Context) {
	s, err := h.svc.GetMasked()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *SettingHandler) Update(c *gin.Context) {
	var body models.Setting
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	body.ID = 1
	if err := h.svc.Update(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}