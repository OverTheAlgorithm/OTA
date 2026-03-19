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
	repo        collector.HistoryRepository
	categoryRepo collector.CategoryRepository
	brainCatRepo collector.BrainCategoryRepository
	authMW      gin.HandlerFunc
}

func NewContextHistoryHandler(repo collector.HistoryRepository, authMW gin.HandlerFunc) *ContextHistoryHandler {
	return &ContextHistoryHandler{repo: repo, authMW: authMW}
}

// WithCategoryRepo sets the category repository for the /categories endpoint.
func (h *ContextHistoryHandler) WithCategoryRepo(catRepo collector.CategoryRepository, brainCatRepo collector.BrainCategoryRepository) *ContextHistoryHandler {
	h.categoryRepo = catRepo
	h.brainCatRepo = brainCatRepo
	return h
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

// GetRecentTopics returns up to 3 random topics from the latest collection run.
// Public endpoint — used on the landing page.
func (h *ContextHistoryHandler) GetRecentTopics(c *gin.Context) {
	topics, err := h.repo.GetRecentTopics(c.Request.Context(), 3)
	if err != nil {
		log.Printf("get recent topics error: %v", err)
		c.JSON(http.StatusOK, gin.H{"data": []any{}})
		return
	}
	if topics == nil {
		topics = []collector.TopicPreview{}
	}
	c.JSON(http.StatusOK, gin.H{"data": topics})
}

// GetAllTopics returns paginated topics with optional category/brain_category filter.
// Public endpoint — used on the all-news page.
func (h *ContextHistoryHandler) GetAllTopics(c *gin.Context) {
	filterType := c.Query("filter_type")
	filterValue := c.Query("filter_value")

	// Validate filter type
	if filterType != "" && filterType != "category" && filterType != "brain_category" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filter_type"})
		return
	}

	limit := 12
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

	topics, hasMore, err := h.repo.GetAllTopics(c.Request.Context(), filterType, filterValue, limit, offset)
	if err != nil {
		log.Printf("get all topics error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if topics == nil {
		topics = []collector.TopicPreview{}
	}
	c.JSON(http.StatusOK, gin.H{"data": topics, "has_more": hasMore})
}

// GetCategories returns all categories and brain categories for the filter UI.
// Public endpoint.
func (h *ContextHistoryHandler) GetCategories(c *gin.Context) {
	ctx := c.Request.Context()

	var categories []collector.Category
	if h.categoryRepo != nil {
		var err error
		categories, err = h.categoryRepo.GetAllCategories(ctx)
		if err != nil {
			log.Printf("get categories error: %v", err)
			categories = nil
		}
	}
	if categories == nil {
		categories = []collector.Category{}
	}

	var brainCategories []collector.BrainCategory
	if h.brainCatRepo != nil {
		var err error
		brainCategories, err = h.brainCatRepo.GetAll(ctx)
		if err != nil {
			log.Printf("get brain categories error: %v", err)
			brainCategories = nil
		}
	}
	if brainCategories == nil {
		brainCategories = []collector.BrainCategory{}
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"categories":       categories,
		"brain_categories": brainCategories,
	}})
}

func (h *ContextHistoryHandler) RegisterRoutes(group *gin.RouterGroup) {
	// Public: topic detail page linked from email
	group.GET("/topic/:id", h.GetTopicByID)

	// Public: recent topics for landing page
	group.GET("/recent", h.GetRecentTopics)

	// Public: all topics with pagination + filter
	group.GET("/topics", h.GetAllTopics)

	// Public: categories + brain categories for filter UI
	group.GET("/categories", h.GetCategories)

	// Auth-required: personal history
	group.GET("/history", h.authMW, h.GetHistory)
}
