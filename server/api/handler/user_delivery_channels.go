package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ota/domain/delivery"
)

// UserDeliveryChannelsHandler handles user delivery channel preferences
type UserDeliveryChannelsHandler struct {
	repo delivery.Repository
}

// NewUserDeliveryChannelsHandler creates a new handler
func NewUserDeliveryChannelsHandler(repo delivery.Repository) *UserDeliveryChannelsHandler {
	return &UserDeliveryChannelsHandler{repo: repo}
}

// RegisterRoutes registers the routes for this handler
func (h *UserDeliveryChannelsHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/delivery-channels", h.GetChannelPreferences)
	group.PUT("/delivery-channels", h.UpdateChannelPreferences)
}

// ChannelPreferenceResponse represents a channel preference in the API
type ChannelPreferenceResponse struct {
	Channel string `json:"channel"` // "email", "kakao", etc.
	Enabled bool   `json:"enabled"`
}

// UpdateChannelPreferencesRequest represents the request to update channel preferences
type UpdateChannelPreferencesRequest struct {
	Channels []ChannelPreferenceResponse `json:"channels"`
}

// GetChannelPreferences returns the user's current channel preferences
// GET /api/v1/user/delivery-channels
func (h *UserDeliveryChannelsHandler) GetChannelPreferences(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	channels, err := h.repo.GetUserDeliveryChannels(c.Request.Context(), userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get channel preferences"})
		return
	}

	// Convert to response format
	response := make([]ChannelPreferenceResponse, 0, len(channels))
	for _, ch := range channels {
		response = append(response, ChannelPreferenceResponse{
			Channel: string(ch.Channel),
			Enabled: ch.Enabled,
		})
	}

	// If user has no preferences yet, return empty array
	if len(response) == 0 {
		response = []ChannelPreferenceResponse{}
	}

	c.JSON(http.StatusOK, gin.H{
		"channels": response,
	})
}

// UpdateChannelPreferences updates the user's channel preferences
// PUT /api/v1/user/delivery-channels
func (h *UserDeliveryChannelsHandler) UpdateChannelPreferences(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req UpdateChannelPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
	}

	// Validate channels
	validChannels := map[string]bool{
		"email":    true,
		"kakao":    true,
		"telegram": true,
		"sms":      true,
		"push":     true,
	}

	for _, ch := range req.Channels {
		if !validChannels[ch.Channel] {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid channel: " + ch.Channel,
				"valid_channels": []string{"email", "kakao", "telegram", "sms", "push"},
			})
			return
		}
	}

	// Upsert each channel preference
	ctx := c.Request.Context()
	for _, ch := range req.Channels {
		channelPref := delivery.UserDeliveryChannel{
			ID:        uuid.New().String(),
			UserID:    userID.(string),
			Channel:   delivery.DeliveryChannel(ch.Channel),
			Enabled:   ch.Enabled,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if err := h.repo.UpsertUserDeliveryChannel(ctx, channelPref); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update channel preferences"})
			return
		}
	}

	// Return updated preferences
	updatedChannels, err := h.repo.GetUserDeliveryChannels(ctx, userID.(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get updated preferences"})
		return
	}

	response := make([]ChannelPreferenceResponse, 0, len(updatedChannels))
	for _, ch := range updatedChannels {
		response = append(response, ChannelPreferenceResponse{
			Channel: string(ch.Channel),
			Enabled: ch.Enabled,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "channel preferences updated",
		"channels": response,
	})
}
