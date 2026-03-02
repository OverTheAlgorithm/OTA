package handler

import (
	"context"
	"log"
	"net/http"
	"strconv"

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
// When uid+rid are provided, attempts to award points and returns earn_result
// in the response. Topic data is always returned regardless of point earning outcome.
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

	// earnResult is only set when uid+rid are provided (i.e. opened from email link).
	// nil means no point earning was attempted.
	type earnResult struct {
		Attempted    bool `json:"attempted"`
		Earned       bool `json:"earned"`
		PointsEarned int  `json:"points_earned"`
		LeveledUp    bool `json:"leveled_up"`
		NewLevel     int  `json:"new_level"`
	}
	var earn *earnResult

	rid, uid := c.Query("rid"), c.Query("uid")
	if rid != "" && uid != "" {
		earn = &earnResult{Attempted: true}
		runID, errR := uuid.Parse(rid)
		if errR != nil {
			log.Printf("invalid rid format: %v", errR)
		} else {
			isToday, err := h.repo.IsRunCreatedToday(c.Request.Context(), runID)
			if err != nil {
				log.Printf("failed to check run creation date: %v", err)
			} else if isToday {
				subs, err := h.subGetter.GetSubscriptions(c.Request.Context(), uid)
				if err != nil {
					log.Printf("failed to get subscriptions: %v", err)
				} else {
					preferred := level.IsPreferredCategory(topic.Category, subs)
					result, earnErr := h.levelService.EarnPoint(c.Request.Context(), uid, runID, topic.ID, preferred)
					if earnErr != nil {
						log.Printf("earn point error: %v", earnErr)
					} else if result.Earned {
						earn.Earned = true
						earn.PointsEarned = result.PointsEarned
						earn.LeveledUp = result.LeveledUp
						earn.NewLevel = result.Level
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": topic, "earn_result": earn})
}

func (h *ContextHistoryHandler) RegisterRoutes(group *gin.RouterGroup) {
	// Public: topic detail page linked from email
	group.GET("/topic/:id", h.GetTopicByID)

	// Auth-required: personal history
	group.GET("/history", h.authMW, h.GetHistory)
}
