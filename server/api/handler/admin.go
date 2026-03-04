package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ota/domain/collector"
	"ota/domain/delivery"
	"ota/domain/level"
)

// MockItemCreator creates a mock over_the_algorithm context item for testing.
type MockItemCreator interface {
	CreateMockOTAItem(ctx context.Context) (uuid.UUID, error)
}

type AdminHandler struct {
	collectorService     *collector.Service
	slackWebhookURL      string
	brainCategoryHandler *BrainCategoryHandler
	levelService         *level.Service
	mockItemCreator      MockItemCreator
	deliveryService      *delivery.Service
}

func NewAdminHandler(collectorService *collector.Service, slackWebhookURL string, brainCatHandler *BrainCategoryHandler) *AdminHandler {
	return &AdminHandler{
		collectorService:     collectorService,
		slackWebhookURL:      slackWebhookURL,
		brainCategoryHandler: brainCatHandler,
	}
}

func (h *AdminHandler) WithLevelService(svc *level.Service) *AdminHandler {
	h.levelService = svc
	return h
}

func (h *AdminHandler) WithMockItemCreator(c MockItemCreator) *AdminHandler {
	h.mockItemCreator = c
	return h
}

func (h *AdminHandler) WithDeliveryService(svc *delivery.Service) *AdminHandler {
	h.deliveryService = svc
	return h
}

// TriggerCollection returns 202 immediately and runs collection in the background.
// The result (or error) is posted to the configured Slack webhook when done.
func (h *AdminHandler) TriggerCollection(c *gin.Context) {
	log.Println("manual collection triggered (async)")
	c.JSON(http.StatusAccepted, gin.H{"message": "collection started"})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()

		result, err := h.collectorService.CollectFromSources(ctx)
		if err != nil {
			log.Printf("collection failed: %v", err)
			h.notifySlack(fmt.Sprintf(":x: *Collection failed*\n```%v```", err))
			return
		}

		log.Printf("collection completed: run_id=%s, items=%d", result.Run.ID, len(result.Items))
		h.notifySlack(fmt.Sprintf(
			":white_check_mark: *Collection completed*\nrun_id: `%s`\nitem_count: `%d`",
			result.Run.ID, len(result.Items),
		))
	}()
}

func (h *AdminHandler) notifySlack(text string) {
	if h.slackWebhookURL == "" {
		return
	}

	payload, _ := json.Marshal(map[string]string{"text": text})
	resp, err := http.Post(h.slackWebhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("slack notification failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("slack notification returned status %d", resp.StatusCode)
	}
}

// SetLevelCoins handles POST /api/v1/admin/level/set-coins
// Directly sets the authenticated user's coins for testing purposes.
func (h *AdminHandler) SetLevelCoins(c *gin.Context) {
	if h.levelService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "level service not available"})
		return
	}

	userID := c.GetString("userID")

	var req struct {
		Coins int `json:"coins" binding:"min=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "coins must be >= 0"})
		return
	}

	info, err := h.levelService.SetCoins(c.Request.Context(), userID, req.Coins)
	if err != nil {
		log.Printf("set level coins error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": info})
}

// CreateMockOTAItem handles POST /api/v1/admin/level/create-mock-item
// Creates a fake over_the_algorithm context item for testing level progression.
// Visit /topic/:id with the returned item_id to earn a point.
func (h *AdminHandler) CreateMockOTAItem(c *gin.Context) {
	if h.mockItemCreator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mock item creator not available"})
		return
	}

	itemID, err := h.mockItemCreator.CreateMockOTAItem(c.Request.Context())
	if err != nil {
		log.Printf("create mock OTA item error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"item_id": itemID.String(),
		"url":     "/topic/" + itemID.String(),
	})
}

// SendTestEmail handles POST /api/v1/admin/delivery/send-test
// Sends the latest briefing email to the authenticated admin for testing.
func (h *AdminHandler) SendTestEmail(c *gin.Context) {
	if h.deliveryService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "delivery service not available"})
		return
	}

	userID := c.GetString("userID")
	result, err := h.deliveryService.ForceDeliverToUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success_count": result.SuccessCount,
		"skipped_count": result.SkippedCount,
		"failure_count": result.FailureCount,
		"errors":        result.DeliveryErrors,
	})
}

func (h *AdminHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/collect", h.TriggerCollection)
	group.POST("/delivery/send-test", h.SendTestEmail)
	group.POST("/level/set-coins", h.SetLevelCoins)
	group.POST("/level/create-mock-item", h.CreateMockOTAItem)

	if h.brainCategoryHandler != nil {
		bcGroup := group.Group("/brain-categories")
		h.brainCategoryHandler.RegisterAdminRoutes(bcGroup)
	}
}
