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
	service        *push.Service
	optionalAuthMW gin.HandlerFunc
	authMW         gin.HandlerFunc
}

func NewPushHandler(service *push.Service, optionalAuthMW, authMW gin.HandlerFunc) *PushHandler {
	return &PushHandler{service: service, optionalAuthMW: optionalAuthMW, authMW: authMW}
}

// RegisterToken registers a device push token.
// If the request includes a valid JWT, the token is linked to the user.
// Otherwise, it is stored anonymously (user_id = NULL).
func (h *PushHandler) RegisterToken(c *gin.Context) {
	userID := c.GetString("userID") // empty if no auth
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

// UnlinkToken removes the user association from a device push token.
// The token row is preserved for anonymous push delivery.
// Requires authentication.
func (h *PushHandler) UnlinkToken(c *gin.Context) {
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

	if err := h.service.UnlinkToken(c.Request.Context(), userID, token); err != nil {
		slog.Error("unlink push token error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (h *PushHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("", h.optionalAuthMW, h.RegisterToken)
	group.DELETE("", h.authMW, h.UnlinkToken)
}
