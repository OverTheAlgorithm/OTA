package handler

import (
	"log"
	"net/http"

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

	result, err := h.collectorService.Collect(c.Request.Context())
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
