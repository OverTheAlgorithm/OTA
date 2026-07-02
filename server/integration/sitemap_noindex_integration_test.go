package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ota/api/handler"
	"ota/domain/collector"
	"ota/storage"
)

type sitemapIntegrationAdapter struct {
	repo *storage.SitemapRepository
}

func (a *sitemapIntegrationAdapter) GetAllTopicIDs(ctx context.Context) ([]handler.TopicEntry, error) {
	rows, err := a.repo.GetAllTopicRows(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]handler.TopicEntry, len(rows))
	for i, r := range rows {
		entries[i] = handler.TopicEntry{ID: r.ID, CreatedAt: r.CreatedAt}
	}
	return entries, nil
}

func (a *sitemapIntegrationAdapter) GetAllEditorPostEntries(ctx context.Context) ([]handler.EditorPostEntry, error) {
	return nil, nil
}

func TestSitemapAndNoIndexIntegration(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	ctx := context.Background()

	// 1. Create a dummy collection run
	runID := uuid.New()
	collectorRepo := storage.NewCollectorRepository(db.Pool)
	if err := collectorRepo.CreateRun(ctx, collector.CollectionRun{
		ID:        runID,
		StartedAt: time.Now().UTC(),
		Status:    collector.RunStatusRunning,
	}); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := collectorRepo.CompleteRun(ctx, runID, collector.RunStatusSuccess, nil, nil); err != nil {
		t.Fatalf("complete run: %v", err)
	}

	// 2. Insert two topics: one new (2 days old), one old (10 days old)
	topicNewID := uuid.New()
	topicOldID := uuid.New()
	now := time.Now().UTC()

	// We seed references >= 1 to satisfy minRefs=1 in new storage.NewSitemapRepository
	q := `
		INSERT INTO context_items
		    (id, collection_run_id, category, brain_category, rank,
		     topic, summary, detail, details, buzz_score, sources, priority, created_at)
		VALUES 
			($1, $2, 'general', NULL, 1, 'New Topic', 'Summary', 'Detail', '[]', 10, '["https://a.com"]'::jsonb, 'none', $3),
			($4, $2, 'general', NULL, 2, 'Old Topic', 'Summary', 'Detail', '[]', 10, '["https://b.com"]'::jsonb, 'none', $5)
	`
	if _, err := db.Pool.Exec(ctx, q, topicNewID, runID, now.Add(-2*24*time.Hour), topicOldID, now.Add(-10*24*time.Hour)); err != nil {
		t.Fatalf("failed to insert test topics: %v", err)
	}

	// 3. Setup Sitemap Handler and History Handler (using MaxAgeDays = 7)
	sitemapRepo := storage.NewSitemapRepository(db.Pool, 1) // MinReferences = 1
	sitemapHandler := handler.NewSitemapHandler(&sitemapIntegrationAdapter{sitemapRepo}, "https://wizletter.com", 7)

	historyRepo := storage.NewHistoryRepository(db.Pool, 1)
	historyHandler := handler.NewContextHistoryHandler(historyRepo, func(c *gin.Context) { c.Next() }).WithMaxAgeDays(7)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/sitemap.xml", sitemapHandler.GetSitemap)
	r.GET("/topic/:id", historyHandler.GetTopicByID)

	// 4. Test Sitemap Filtering
	wSitemap := httptest.NewRecorder()
	reqSitemap := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	r.ServeHTTP(wSitemap, reqSitemap)

	if wSitemap.Code != http.StatusOK {
		t.Fatalf("expected sitemap status 200, got %d", wSitemap.Code)
	}

	sitemapBody := wSitemap.Body.String()
	newTopicURL := fmt.Sprintf("https://wizletter.com/topic/%s", topicNewID)
	oldTopicURL := fmt.Sprintf("https://wizletter.com/topic/%s", topicOldID)

	if !strings.Contains(sitemapBody, newTopicURL) {
		t.Errorf("expected sitemap to contain new topic URL: %s", newTopicURL)
	}
	if strings.Contains(sitemapBody, oldTopicURL) {
		t.Errorf("expected sitemap NOT to contain old topic URL: %s", oldTopicURL)
	}

	// 5. Test Topic Detail no_index value for NEW topic (should be false)
	wNew := httptest.NewRecorder()
	reqNew := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/topic/%s", topicNewID), nil)
	r.ServeHTTP(wNew, reqNew)

	if wNew.Code != http.StatusOK {
		t.Fatalf("expected new topic status 200, got %d", wNew.Code)
	}

	var respNew map[string]interface{}
	if err := json.Unmarshal(wNew.Body.Bytes(), &respNew); err != nil {
		t.Fatalf("failed to unmarshal new topic response: %v", err)
	}
	dataNew, _ := respNew["data"].(map[string]interface{})
	gotNoIdxNew, _ := dataNew["no_index"].(bool)
	if gotNoIdxNew {
		t.Errorf("expected no_index = false for new topic, got true")
	}

	// 6. Test Topic Detail no_index value for OLD topic (should be true)
	wOld := httptest.NewRecorder()
	reqOld := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/topic/%s", topicOldID), nil)
	r.ServeHTTP(wOld, reqOld)

	if wOld.Code != http.StatusOK {
		t.Fatalf("expected old topic status 200, got %d", wOld.Code)
	}

	var respOld map[string]interface{}
	if err := json.Unmarshal(wOld.Body.Bytes(), &respOld); err != nil {
		t.Fatalf("failed to unmarshal old topic response: %v", err)
	}
	dataOld, _ := respOld["data"].(map[string]interface{})
	gotNoIdxOld, _ := dataOld["no_index"].(bool)
	if !gotNoIdxOld {
		t.Errorf("expected no_index = true for old topic, got false")
	}
}
