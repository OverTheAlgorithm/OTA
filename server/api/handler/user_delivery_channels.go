package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ota/domain/delivery"
	userDomain "ota/domain/user"
)

var validChannels = map[string]bool{
	"email":    true,
	"kakao":    true,
	"telegram": true,
	"sms":      true,
	"push":     true,
}

// UserDeliveryChannelsHandler handles user delivery channel preferences and status
type UserDeliveryChannelsHandler struct {
	repo            delivery.Repository
	deliveryService *delivery.Service
	userRepo        userDomain.Repository
}

// NewUserDeliveryChannelsHandler creates a new handler
func NewUserDeliveryChannelsHandler(repo delivery.Repository, deliveryService *delivery.Service, userRepo userDomain.Repository) *UserDeliveryChannelsHandler {
	return &UserDeliveryChannelsHandler{repo: repo, deliveryService: deliveryService, userRepo: userRepo}
}

// ChannelDeliveryStatusResponse represents per-channel delivery status
type ChannelDeliveryStatusResponse struct {
	Channel      string    `json:"channel"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	RetryCount   int       `json:"retry_count"`
	LastAttempt  time.Time `json:"last_attempt"`
}

// RegisterRoutes registers the routes for this handler
func (h *UserDeliveryChannelsHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/delivery-channels", h.GetChannelPreferences)
	group.PUT("/delivery-channels", h.UpdateChannelPreferences)
	group.GET("/delivery-status", h.GetDeliveryStatus)
	group.PUT("/nickname", h.UpdateNickname)
	group.POST("/nickname-warning/dismiss", h.DismissNicknameWarning)
}

// UpdateNickname handles PUT /api/v1/user/nickname.
// Persists a normalised nickname and advances nickname_state to 'custom',
// which locks the nickname against future Kakao-login overwrites.
func (h *UserDeliveryChannelsHandler) UpdateNickname(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req struct {
		Nickname string `json:"nickname" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "요청 형식이 올바르지 않습니다"})
		return
	}

	normalised, err := userDomain.NormaliseNickname(req.Nickname)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userRepo.UpdateNickname(c.Request.Context(), userID, normalised); err != nil {
		slog.Error("update nickname", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "닉네임 변경 중 오류가 발생했습니다"})
		return
	}

	u, err := h.userRepo.FindByID(c.Request.Context(), userID)
	if err != nil {
		slog.Error("reload user after nickname update", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "유저 정보를 다시 불러올 수 없습니다"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": u})
}

// DismissNicknameWarning handles POST /api/v1/user/nickname-warning/dismiss.
// Advances nickname_state from 'default' to 'acknowledged' so the comment
// composer stops prompting. Idempotent for any state other than 'default'.
func (h *UserDeliveryChannelsHandler) DismissNicknameWarning(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.userRepo.AcknowledgeNicknameWarning(c.Request.Context(), userID); err != nil {
		slog.Error("dismiss nickname warning", "user_id", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "처리 중 오류가 발생했습니다"})
		return
	}

	u, err := h.userRepo.FindByID(c.Request.Context(), userID)
	if err != nil {
		// Return 204 if we can't reload — the state change already
		// happened, the client can poll for the updated state.
		_ = errors.Unwrap(err)
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": u})
}

// GetDeliveryStatus returns the user's latest delivery status per channel
// GET /api/v1/user/delivery-status
func (h *UserDeliveryChannelsHandler) GetDeliveryStatus(c *gin.Context) {
	userID := c.GetString("userID")

	logs, err := h.deliveryService.GetUserDeliveryStatus(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get delivery status"})
		return
	}

	statuses := make([]ChannelDeliveryStatusResponse, 0, len(logs))
	for _, log := range logs {
		statuses = append(statuses, ChannelDeliveryStatusResponse{
			Channel:      string(log.Channel),
			Status:       string(log.Status),
			ErrorMessage: log.ErrorMessage,
			RetryCount:   log.RetryCount,
			LastAttempt:  log.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": statuses})
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
	userID := c.GetString("userID")

	channels, err := h.repo.GetUserDeliveryChannels(c.Request.Context(), userID)
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
	userID := c.GetString("userID")

	var req UpdateChannelPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request format"})
		return
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

	// Require email verification before enabling email channel.
	for _, ch := range req.Channels {
		if ch.Channel == "email" && ch.Enabled {
			u, err := h.userRepo.FindByID(c.Request.Context(), userID)
			if err != nil || !u.EmailVerified {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "이메일 인증이 필요합니다. 이메일을 먼저 인증해주세요.",
					"code":  "EMAIL_NOT_VERIFIED",
				})
				return
			}
			break
		}
	}

	// Upsert each channel preference
	ctx := c.Request.Context()
	for _, ch := range req.Channels {
		channelPref := delivery.UserDeliveryChannel{
			ID:        uuid.New().String(),
			UserID:    userID,
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
	updatedChannels, err := h.repo.GetUserDeliveryChannels(ctx, userID)
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
