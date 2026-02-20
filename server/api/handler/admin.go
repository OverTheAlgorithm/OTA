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

	"ota/domain/collector"
)

type AdminHandler struct {
	collectorService *collector.Service
	slackWebhookURL  string
}

func NewAdminHandler(collectorService *collector.Service, slackWebhookURL string) *AdminHandler {
	return &AdminHandler{
		collectorService: collectorService,
		slackWebhookURL:  slackWebhookURL,
	}
}

// TriggerCollection returns 202 immediately and runs collection in the background.
// The result (or error) is posted to the configured Slack webhook when done.
func (h *AdminHandler) TriggerCollection(c *gin.Context) {
	log.Println("manual collection triggered (async)")
	c.JSON(http.StatusAccepted, gin.H{"message": "collection started"})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()

		result, err := h.collectorService.Collect(ctx)
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

func (h *AdminHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/collect", h.TriggerCollection)
}
