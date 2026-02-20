package handler

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ota/domain/collector"
)

type AdminHandler struct {
	collectorService *collector.Service
}

func NewAdminHandler(collectorService *collector.Service) *AdminHandler {
	return &AdminHandler{
		collectorService: collectorService,
	}
}

func (h *AdminHandler) TriggerCollection(c *gin.Context) {
	log.Println("manual collection triggered")

	// Use a detached 1-hour context so that thinking-model AI calls are not
	// cancelled by the HTTP request lifecycle or any proxy timeout.
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()

	result, err := h.collectorService.Collect(ctx)
	if err != nil {
		log.Printf("collection failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "collection failed", "details": err.Error()})
		return
	}

	log.Printf("collection completed: run_id=%s, items=%d", result.Run.ID, len(result.Items))

	c.JSON(http.StatusOK, gin.H{
		"message": "collection completed",
		"data": gin.H{
			"run_id":     result.Run.ID,
			"item_count": len(result.Items),
			"items":      result.Items,
		},
	})
}

func (h *AdminHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/collect", h.TriggerCollection)
}
