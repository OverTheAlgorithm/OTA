package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"ota/domain/poll"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PollService is the subset of *poll.Service used by the public handler.
type PollService interface {
	GetForUser(ctx context.Context, userID string, contextItemID uuid.UUID) (*poll.PollForUser, error)
	Vote(ctx context.Context, userID string, contextItemID uuid.UUID, optionIndex int) error
}

// PollHandler handles public poll reads and votes.
type PollHandler struct {
	svc            PollService
	authMW         gin.HandlerFunc
	optionalAuthMW gin.HandlerFunc
}

// NewPollHandler creates a new PollHandler.
func NewPollHandler(svc PollService, authMW, optionalAuthMW gin.HandlerFunc) *PollHandler {
	return &PollHandler{svc: svc, authMW: authMW, optionalAuthMW: optionalAuthMW}
}

// GetPoll handles GET /polls/:context_item_id. Public; optional auth.
func (h *PollHandler) GetPoll(c *gin.Context) {
	id, err := uuid.Parse(c.Param("context_item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}
	userID := c.GetString("userID")
	result, err := h.svc.GetForUser(c.Request.Context(), userID, id)
	if err != nil {
		slog.Error("get poll error", "item_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no poll for this article"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// Vote handles POST /polls/:context_item_id/vote. Auth required.
// On 409 (already voted), the body embeds the current server-authoritative poll under "data".
func (h *PollHandler) Vote(c *gin.Context) {
	id, err := uuid.Parse(c.Param("context_item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}
	userID := c.GetString("userID")

	var req struct {
		OptionIndex int `json:"option_index"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.svc.Vote(c.Request.Context(), userID, id, req.OptionIndex); err != nil {
		switch {
		case errors.Is(err, poll.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "no poll for this article"})
		case errors.Is(err, poll.ErrInvalidOption):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid option_index"})
		case errors.Is(err, poll.ErrAlreadyVoted):
			if refreshed, rerr := h.svc.GetForUser(c.Request.Context(), userID, id); rerr == nil && refreshed != nil {
				c.JSON(http.StatusConflict, gin.H{"error": "already voted", "data": refreshed})
				return
			}
			c.JSON(http.StatusConflict, gin.H{"error": "already voted"})
		default:
			slog.Error("vote error", "user_id", userID, "item_id", id, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	refreshed, err := h.svc.GetForUser(c.Request.Context(), userID, id)
	if err != nil || refreshed == nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": refreshed})
}

// RegisterRoutes wires the public poll routes under /api/v1/polls.
func (h *PollHandler) RegisterRoutes(group *gin.RouterGroup) {
	if h.optionalAuthMW != nil {
		group.GET("/:context_item_id", h.optionalAuthMW, h.GetPoll)
	} else {
		group.GET("/:context_item_id", h.GetPoll)
	}
	group.POST("/:context_item_id/vote", h.authMW, h.Vote)
}
