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

// AdminPollService is the subset of *poll.Service used by admin ops.
type AdminPollService interface {
	UpdatePoll(ctx context.Context, contextItemID uuid.UUID, question string, options []string) error
	DeletePoll(ctx context.Context, contextItemID uuid.UUID) error
}

// AdminPollHandler handles admin-only poll edits and deletes.
type AdminPollHandler struct {
	svc AdminPollService
}

// NewAdminPollHandler creates a new AdminPollHandler.
func NewAdminPollHandler(svc AdminPollService) *AdminPollHandler {
	return &AdminPollHandler{svc: svc}
}

type adminUpdatePollRequest struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
}

// UpdatePoll handles PUT /admin/polls/context/:context_item_id.
func (h *AdminPollHandler) UpdatePoll(c *gin.Context) {
	id, err := uuid.Parse(c.Param("context_item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}
	var req adminUpdatePollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := h.svc.UpdatePoll(c.Request.Context(), id, req.Question, req.Options); err != nil {
		switch {
		case errors.Is(err, poll.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		case errors.Is(err, poll.ErrInvalidOption):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid poll payload"})
		default:
			slog.Error("admin update poll error", "item_id", id, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"updated": true}})
}

// DeletePoll handles DELETE /admin/polls/context/:context_item_id.
func (h *AdminPollHandler) DeletePoll(c *gin.Context) {
	id, err := uuid.Parse(c.Param("context_item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid context_item_id"})
		return
	}
	if err := h.svc.DeletePoll(c.Request.Context(), id); err != nil {
		slog.Error("admin delete poll error", "item_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true}})
}

// RegisterRoutes wires admin routes under /api/v1/admin/polls. Middlewares applied at group level.
func (h *AdminPollHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.PUT("/context/:context_item_id", h.UpdatePoll)
	group.DELETE("/context/:context_item_id", h.DeletePoll)
}
