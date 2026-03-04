package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"ota/domain/level"
)

type LevelHandler struct {
	service *level.Service
	authMW  gin.HandlerFunc
}

func NewLevelHandler(service *level.Service, authMW gin.HandlerFunc) *LevelHandler {
	return &LevelHandler{service: service, authMW: authMW}
}

// GetLevel handles GET /api/v1/level
func (h *LevelHandler) GetLevel(c *gin.Context) {
	userID := c.GetString("userID")
	info, err := h.service.GetLevel(c.Request.Context(), userID)
	if err != nil {
		log.Printf("get level error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

// EarnCoin handles POST /api/v1/level/earn
func (h *LevelHandler) EarnCoin(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		ContextItemID string `json:"context_item_id" binding:"required"`
		RunID         string `json:"run_id" binding:"required"`
		Preferred     bool   `json:"preferred"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "context_item_id and run_id are required"})
		return
	}

	itemID, err := uuid.Parse(req.ContextItemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}

	runID, err := uuid.Parse(req.RunID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid run_id"})
		return
	}

	result, err := h.service.EarnCoin(c.Request.Context(), userID, runID, itemID, req.Preferred)
	if err != nil {
		log.Printf("earn coin error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (h *LevelHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("", h.authMW, h.GetLevel)      // GET /api/v1/level
	group.POST("/earn", h.authMW, h.EarnCoin) // POST /api/v1/level/earn
}
