package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ota/domain/delivery"
)

// DeliveryHandler handles delivery-related HTTP requests
type DeliveryHandler struct {
	service        *delivery.Service
	authMiddleware gin.HandlerFunc
}

// NewDeliveryHandler creates a new delivery handler
func NewDeliveryHandler(service *delivery.Service, authMiddleware gin.HandlerFunc) *DeliveryHandler {
	return &DeliveryHandler{
		service:        service,
		authMiddleware: authMiddleware,
	}
}

// RegisterRoutes registers delivery-related routes
func (h *DeliveryHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/trigger", h.TriggerDelivery)
	group.POST("/send", h.authMiddleware, h.SendToCurrentUser)
}

// TriggerDelivery manually triggers message delivery to all eligible users
// POST /api/v1/delivery/trigger
func (h *DeliveryHandler) TriggerDelivery(c *gin.Context) {
	result, err := h.service.DeliverAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_users":   result.TotalUsers,
		"success_count": result.SuccessCount,
		"failure_count": result.FailureCount,
		"skipped_count": result.SkippedCount,
		"failed_users":  result.FailedUsers,
		"errors":        result.DeliveryErrors,
	})
}

// SendToCurrentUser delivers the latest briefing to the authenticated user on-demand
// POST /api/v1/delivery/send
func (h *DeliveryHandler) SendToCurrentUser(c *gin.Context) {
	userID := c.GetString("userID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	result, err := h.service.DeliverToUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"success_count": result.SuccessCount,
			"failure_count": result.FailureCount,
			"skipped_count": result.SkippedCount,
			"errors":        result.DeliveryErrors,
		},
	})
}
