package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"ota/domain/collector"
	"ota/domain/quiz"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ContextHistoryHandler struct {
	repo           collector.HistoryRepository
	categoryRepo   collector.CategoryRepository
	brainCatRepo   collector.BrainCategoryRepository
	quizSvc        *quiz.Service
	authMW         gin.HandlerFunc
	optionalAuthMW gin.HandlerFunc
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

// WithQuizService sets the quiz service for bundling quiz data with topic detail responses.
func (h *ContextHistoryHandler) WithQuizService(quizSvc *quiz.Service, optionalAuthMW gin.HandlerFunc) *ContextHistoryHandler {
	h.quizSvc = quizSvc
	h.optionalAuthMW = optionalAuthMW
	return h
}

// topicResponse composes TopicDetail with quiz data at the handler level to avoid circular imports.
type topicResponse struct {
	collector.TopicDetail
	HasQuiz bool              `json:"has_quiz"`
	Quiz    *quiz.QuizForUser `json:"quiz"`
}

func (h *ContextHistoryHandler) GetHistory(c *gin.Context) {
	userID := c.GetString("userID")

	limit, offset := parsePageParams(c, 10, 50)

	entries, hasMore, err := h.repo.GetHistoryForUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		slog.Error("get context history error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries, "has_more": hasMore})
}

// GetTopicByID returns the full detail for a single context item.
// Public endpoint with optional auth — quiz data bundled when user is logged in and eligible.
func (h *ContextHistoryHandler) GetTopicByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	topic, err := h.repo.GetContextItemByID(c.Request.Context(), id)
	if err != nil {
		slog.Error("get topic by id error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if topic == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "topic not found"})
		return
	}

	resp := topicResponse{
		TopicDetail: *topic,
		HasQuiz:     topic.HasQuiz,
		Quiz:        nil,
	}

	// Bundle quiz data if user is logged in and eligible.
	userID := c.GetString("userID")
	if userID != "" && h.quizSvc != nil {
		quizForUser, err := h.quizSvc.GetQuizForUser(c.Request.Context(), userID, id)
		if err != nil && !errors.Is(err, quiz.ErrNotEarned) && !errors.Is(err, quiz.ErrAlreadyAttempted) {
			slog.Warn("get quiz for user error", "user_id", userID, "item_id", id, "error", err)
		}
		resp.Quiz = quizForUser
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// GetRecentTopics returns up to 3 random topics from the latest collection run.
// Public endpoint — used on the landing page.
func (h *ContextHistoryHandler) GetRecentTopics(c *gin.Context) {
	topics, err := h.repo.GetRecentTopics(c.Request.Context(), 3)
	if err != nil {
		slog.Warn("get recent topics error", "error", err)
		c.JSON(http.StatusOK, gin.H{"data": []any{}})
		return
	}
	if topics == nil {
		topics = []collector.TopicPreview{}
	}
	c.JSON(http.StatusOK, gin.H{"data": topics})
}

// GetLatestRunTopics returns all topics from the latest successful collection run.
// Public endpoint — used on the home (latest news) page.
func (h *ContextHistoryHandler) GetLatestRunTopics(c *gin.Context) {
	topics, err := h.repo.GetLatestRunTopics(c.Request.Context())
	if err != nil {
		slog.Warn("get latest run topics error", "error", err)
		c.JSON(http.StatusOK, gin.H{"data": []collector.TopicPreview{}})
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

	limit, offset := parsePageParams(c, 12, 50)

	topics, hasMore, err := h.repo.GetAllTopics(c.Request.Context(), filterType, filterValue, limit, offset)
	if err != nil {
		slog.Error("get all topics error", "error", err)
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
			slog.Warn("get categories error", "error", err)
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
			slog.Warn("get brain categories error", "error", err)
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
	// Public with optional auth: topic detail page (quiz bundled when logged in)
	if h.optionalAuthMW != nil {
		group.GET("/topic/:id", h.optionalAuthMW, h.GetTopicByID)
	} else {
		group.GET("/topic/:id", h.GetTopicByID)
	}

	// Public: recent topics for landing page
	group.GET("/recent", h.GetRecentTopics)

	// Public: all topics from the latest collection run
	group.GET("/latest", h.GetLatestRunTopics)

	// Public: all topics with pagination + filter
	group.GET("/topics", h.GetAllTopics)

	// Public: categories + brain categories for filter UI
	group.GET("/categories", h.GetCategories)

	// Auth-required: personal history
	group.GET("/history", h.authMW, h.GetHistory)
}
