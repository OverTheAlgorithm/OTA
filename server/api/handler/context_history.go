package handler

import (
	"context"
	"log"
	"net/http"

	"ota/domain/collector"
	"ota/domain/level"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SubscriptionGetter interface {
	GetSubscriptions(ctx context.Context, userID string) ([]string, error)
}

type ContextHistoryHandler struct {
	repo         collector.HistoryRepository
	levelService *level.Service
	subGetter    SubscriptionGetter
	authMW       gin.HandlerFunc
}

func NewContextHistoryHandler(repo collector.HistoryRepository, levelService *level.Service, subGetter SubscriptionGetter, authMW gin.HandlerFunc) *ContextHistoryHandler {
	return &ContextHistoryHandler{repo: repo, levelService: levelService, subGetter: subGetter, authMW: authMW}
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

	rid, uid := c.Query("rid"), c.Query("uid")
	if rid != "" && uid != "" {
		runID, errR := uuid.Parse(rid)
		if errR == nil {
			isToday, err := h.repo.IsRunCreatedToday(c.Request.Context(), runID)
			if err == nil && isToday {
				subs, err := h.subGetter.GetSubscriptions(c.Request.Context(), uid)
				if err == nil {
					preferred := level.IsPreferredCategory(topic.Category, subs)
					_, earnErr := h.levelService.EarnPoint(c.Request.Context(), uid, runID, topic.ID, preferred)
					if earnErr != nil {
						log.Printf("earn point error: %v", earnErr)
					}
				} else {
					log.Printf("failed to get subscriptions for point earning: %v", err)
				}
			} else if err != nil {
				log.Printf("failed to check run creation date: %v", err)
			}
		} else {
			log.Printf("invalid rid format for point earning: %v", errR)
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": topic})
}

func (h *ContextHistoryHandler) RegisterRoutes(group *gin.RouterGroup) {
	// Public: topic detail page linked from email
	group.GET("/topic/:id", h.GetTopicByID)

	// Auth-required: personal history
	group.GET("/history", h.authMW, h.GetHistory)
}
