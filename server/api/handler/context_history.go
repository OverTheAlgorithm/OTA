package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"ota/domain/collector"
)

type ContextHistoryHandler struct {
	repo   collector.HistoryRepository
	authMW gin.HandlerFunc
}

func NewContextHistoryHandler(repo collector.HistoryRepository, authMW gin.HandlerFunc) *ContextHistoryHandler {
	return &ContextHistoryHandler{repo: repo, authMW: authMW}
}

func (h *ContextHistoryHandler) GetHistory(c *gin.Context) {
	userID := c.GetString("userID")
	entries, err := h.repo.GetHistoryForUser(c.Request.Context(), userID)
	if err != nil {
		log.Printf("get context history error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

func (h *ContextHistoryHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.Use(h.authMW)
	group.GET("/history", h.GetHistory)
}
