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

// QuizHandler handles quiz answer submission.
type QuizHandler struct {
	svc      *quiz.Service
	histRepo collector.HistoryRepository
	authMW   gin.HandlerFunc
}

// NewQuizHandler creates a new QuizHandler.
func NewQuizHandler(svc *quiz.Service, histRepo collector.HistoryRepository, authMW gin.HandlerFunc) *QuizHandler {
	return &QuizHandler{svc: svc, histRepo: histRepo, authMW: authMW}
}

// SubmitAnswer handles POST /quiz/:context_item_id.
// Auth required. Returns 403 if earn-gate fails, 409 if already attempted.
func (h *QuizHandler) SubmitAnswer(c *gin.Context) {
	userID := c.GetString("userID")

	idStr := c.Param("context_item_id")
	contextItemID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}

	var req struct {
		AnswerIndex int `json:"answer_index"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Fetch topic name for coin_event memo — best-effort, empty string if not found.
	topicName := ""
	if topic, err := h.histRepo.GetContextItemByID(c.Request.Context(), contextItemID); err == nil && topic != nil {
		topicName = topic.Topic
	}

	result, err := h.svc.SubmitAnswer(c.Request.Context(), userID, contextItemID, req.AnswerIndex, topicName)
	if err != nil {
		switch {
		case errors.Is(err, quiz.ErrNotEarned):
			c.JSON(http.StatusForbidden, gin.H{"error": "coins not yet earned for this article"})
		case errors.Is(err, quiz.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "no quiz for this article"})
		default:
			if isQuizAlreadyAttemptedErr(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "quiz already attempted"})
				return
			}
			slog.Error("submit quiz answer error", "user_id", userID, "item_id", contextItemID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// isQuizAlreadyAttemptedErr detects the duplicate-attempt error from the storage layer.
func isQuizAlreadyAttemptedErr(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "submit answer: save result: save result and award coins: already attempted"
}

func (h *QuizHandler) RegisterRoutes(group *gin.RouterGroup) {
	// POST /api/v1/quiz/:context_item_id — auth required
	group.POST("/:context_item_id", h.authMW, h.SubmitAnswer)
}
