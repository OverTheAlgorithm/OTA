package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AdblockRepository is the interface the handler depends on for persisting adblock status.
type AdblockRepository interface {
	UpdateAdblockStatus(ctx context.Context, userID string, detected bool) error
}

// AdblockHandler handles adblock status reporting.
type AdblockHandler struct {
	repo AdblockRepository
}

// NewAdblockHandler constructs an AdblockHandler.
func NewAdblockHandler(repo AdblockRepository) *AdblockHandler {
	return &AdblockHandler{repo: repo}
}

// ReportAdblock handles POST /api/v1/adblock/report.
// Body: { "detected": bool }
// Requires authentication — userID is set by AuthMiddleware.
func (h *AdblockHandler) ReportAdblock(c *gin.Context) {
	userID := c.GetString("userID")

	var req struct {
		Detected bool `json:"detected"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.repo.UpdateAdblockStatus(c.Request.Context(), userID, req.Detected); err != nil {
		slog.Error("adblock: update status error", "error", err, "userID", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// RegisterRoutes registers the adblock routes on the given group.
// The group prefix is expected to be "adblock".
func (h *AdblockHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("report", h.ReportAdblock)
}
