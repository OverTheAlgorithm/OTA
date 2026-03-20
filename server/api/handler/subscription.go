package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"ota/domain/user"
)

type SubscriptionHandler struct {
	repo   user.SubscriptionRepository
	authMW gin.HandlerFunc
}

func NewSubscriptionHandler(repo user.SubscriptionRepository, authMW gin.HandlerFunc) *SubscriptionHandler {
	return &SubscriptionHandler{repo: repo, authMW: authMW}
}

func (h *SubscriptionHandler) List(c *gin.Context) {
	userID := c.GetString("userID")
	cats, err := h.repo.GetSubscriptions(c.Request.Context(), userID)
	if err != nil {
		slog.Error("get subscriptions error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": cats})
}

func (h *SubscriptionHandler) Add(c *gin.Context) {
	userID := c.GetString("userID")
	var body struct {
		Category string `json:"category" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category is required"})
		return
	}
	if err := h.repo.AddSubscription(c.Request.Context(), userID, body.Category); err != nil {
		slog.Error("add subscription error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (h *SubscriptionHandler) Delete(c *gin.Context) {
	userID := c.GetString("userID")
	category := c.Query("category")
	if category == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category is required"})
		return
	}
	if err := h.repo.DeleteSubscription(c.Request.Context(), userID, category); err != nil {
		slog.Error("delete subscription error", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (h *SubscriptionHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.Use(h.authMW)
	group.GET("", h.List)
	group.POST("", h.Add)
	group.DELETE("", h.Delete)
}
