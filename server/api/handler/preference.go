package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"ota/domain/user"
)

type PreferenceHandler struct {
	repo   user.PreferenceRepository
	authMW gin.HandlerFunc
}

func NewPreferenceHandler(repo user.PreferenceRepository, authMW gin.HandlerFunc) *PreferenceHandler {
	return &PreferenceHandler{repo: repo, authMW: authMW}
}

func (h *PreferenceHandler) Get(c *gin.Context) {
	userID := c.GetString("userID")
	enabled, err := h.repo.GetPreference(c.Request.Context(), userID)
	if err != nil {
		slog.Error("get preference error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"delivery_enabled": enabled}})
}

func (h *PreferenceHandler) Update(c *gin.Context) {
	userID := c.GetString("userID")
	var body struct {
		DeliveryEnabled bool `json:"delivery_enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if err := h.repo.UpsertPreference(c.Request.Context(), userID, body.DeliveryEnabled); err != nil {
		slog.Error("update preference error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (h *PreferenceHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.Use(h.authMW)
	group.GET("", h.Get)
	group.PUT("", h.Update)
}
