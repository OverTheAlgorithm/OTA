package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"ota/domain/push"

	"github.com/gin-gonic/gin"
)

// PushHandler handles push token registration and unregistration.
type PushHandler struct {
	service *push.Service
}

func NewPushHandler(service *push.Service) *PushHandler {
	return &PushHandler{service: service}
}

// RegisterToken registers a device push token for the authenticated user.
func (h *PushHandler) RegisterToken(c *gin.Context) {
	userID := c.GetString("userID")
	var req struct {
		Token    string `json:"token" binding:"required"`
		Platform string `json:"platform"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	if err := h.service.RegisterToken(c.Request.Context(), userID, token, req.Platform); err != nil {
		slog.Error("register push token error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

// UnregisterToken removes a device push token for the authenticated user.
func (h *PushHandler) UnregisterToken(c *gin.Context) {
	userID := c.GetString("userID")
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	token := strings.TrimSpace(req.Token)
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	if err := h.service.UnregisterToken(c.Request.Context(), userID, token); err != nil {
		slog.Error("unregister push token error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (h *PushHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("", h.RegisterToken)
	group.DELETE("", h.UnregisterToken)
}
