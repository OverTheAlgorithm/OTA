package handler

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"ota/domain/collector"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SubscriptionGetter interface {
	GetSubscriptions(ctx context.Context, userID string) ([]string, error)
}

type ContextHistoryHandler struct {
	repo   collector.HistoryRepository
	authMW gin.HandlerFunc
}

func NewContextHistoryHandler(repo collector.HistoryRepository, authMW gin.HandlerFunc) *ContextHistoryHandler {
	return &ContextHistoryHandler{repo: repo, authMW: authMW}
}

func (h *ContextHistoryHandler) GetHistory(c *gin.Context) {
	userID := c.GetString("userID")

	limit := 10
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	entries, hasMore, err := h.repo.GetHistoryForUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		log.Printf("get context history error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries, "has_more": hasMore})
}

// GetTopicByID returns the full detail for a single context item.
// Public endpoint — no auth required (linked from email).
func (h *ContextHistoryHandler) GetTopicByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	topic, err := h.repo.GetContextItemByID(c.Request.Context(), id)
	if err != nil {
		log.Printf("get topic by id error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if topic == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "topic not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": topic})
}

func (h *ContextHistoryHandler) RegisterRoutes(group *gin.RouterGroup) {
	// Public: topic detail page linked from email
	group.GET("/topic/:id", h.GetTopicByID)

	// Auth-required: personal history
	group.GET("/history", h.authMW, h.GetHistory)
}
