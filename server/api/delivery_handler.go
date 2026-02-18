package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ota/domain/delivery"
)

// DeliveryHandler handles delivery-related HTTP requests
type DeliveryHandler struct {
	service *delivery.Service
}

// NewDeliveryHandler creates a new delivery handler
func NewDeliveryHandler(service *delivery.Service) *DeliveryHandler {
	return &DeliveryHandler{
		service: service,
	}
}

// RegisterRoutes registers delivery-related routes
func (h *DeliveryHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/trigger", h.TriggerDelivery)
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
