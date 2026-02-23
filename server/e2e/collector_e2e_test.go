package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"ota/domain/collector"
	"ota/platform/gemini"
	"ota/platform/googlenews"
	"ota/platform/googletrends"
	"ota/storage"
)

// TestCollector_E2E_WithRealAPI tests the complete structured pipeline:
// 1. Real RSS collection (Google Trends + Google News)
// 2. Real Gemini API call for clustering + summarization
// 3. Store in PostgreSQL database
// 4. Verify data integrity
func TestCollector_E2E_WithRealAPI(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "trending_items", "context_items", "collection_runs")

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set - skipping E2E test with real API")
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash-lite"
	}

	t.Logf("Testing E2E structured pipeline (model: %s)", model)

	// Create real collectors + AI client
	aiClient := gemini.NewClient(apiKey, model)
	trendsCollector := googletrends.NewCollector()
	newsCollector := googlenews.NewCollector(googlenews.DefaultTopics())
	aggregator := collector.NewAggregator([]collector.SourceCollector{trendsCollector, newsCollector})

	repo := storage.NewCollectorRepository(db.Pool)
	trendingRepo := storage.NewTrendingItemRepository(db.Pool)
	service := collector.NewService(aiClient, repo)
	service.WithAggregator(aggregator).WithTrendingRepo(trendingRepo)

	// Execute collection
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	t.Log("Running structured pipeline: RSS collection → AI clustering → DB storage...")
	result, err := service.CollectFromSources(ctx)
	if err != nil {
		t.Fatalf("CollectFromSources failed: %v", err)
	}

	// Verify collection run
	if result.Run.Status != collector.RunStatusSuccess {
		t.Errorf("Expected status %s, got %s", collector.RunStatusSuccess, result.Run.Status)
		if result.Run.ErrorMessage != nil {
			t.Errorf("Error message: %s", *result.Run.ErrorMessage)
		}
	}

	if result.Run.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}

	// Verify context items
	if len(result.Items) == 0 {
		t.Fatal("Expected at least 1 context item, got 0")
	}

	t.Logf("Collection succeeded: %d context items", len(result.Items))

	// Verify items have required fields
	categorySeen := make(map[string]bool)
	for i, item := range result.Items {
		if item.Category == "" {
			t.Errorf("Item %d: category is empty", i)
		}
		if item.Rank < 1 {
			t.Errorf("Item %d: rank should be >= 1, got %d", i, item.Rank)
		}
		if item.Topic == "" {
			t.Errorf("Item %d: topic is empty", i)
		}
		if item.Summary == "" {
			t.Errorf("Item %d: summary is empty", i)
		}
		categorySeen[item.Category] = true
		t.Logf("  [%s] Rank %d (buzz:%d): %s", item.Category, item.Rank, item.BuzzScore, item.Topic)
	}

	if !categorySeen["top"] {
		t.Error("Expected at least one item with category 'top'")
	}

	// Verify data was stored in database
	t.Log("Verifying database storage...")

	var dbStatus string
	var dbRawResponse *string
	err = db.Pool.QueryRow(ctx,
		`SELECT status, raw_response FROM collection_runs WHERE id = $1`,
		result.Run.ID,
	).Scan(&dbStatus, &dbRawResponse)
	if err != nil {
		t.Fatalf("Failed to query collection_runs: %v", err)
	}

	if dbStatus != string(collector.RunStatusSuccess) {
		t.Errorf("DB status mismatch: expected %s, got %s", collector.RunStatusSuccess, dbStatus)
	}

	// Verify trending items were persisted
	var trendingCount int
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM trending_items WHERE collection_run_id = $1`,
		result.Run.ID,
	).Scan(&trendingCount)
	if err != nil {
		t.Fatalf("Failed to count trending items: %v", err)
	}
	t.Logf("  Trending items in DB: %d", trendingCount)
	if trendingCount == 0 {
		t.Error("Expected trending items to be persisted in DB")
	}

	// Verify context items in DB
	var contextCount int
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM context_items WHERE collection_run_id = $1`,
		result.Run.ID,
	).Scan(&contextCount)
	if err != nil {
		t.Fatalf("Failed to count context items: %v", err)
	}
	if contextCount != len(result.Items) {
		t.Errorf("DB context item count mismatch: expected %d, got %d", len(result.Items), contextCount)
	}

	t.Logf("E2E test passed: RSS(%d items) → AI → DB(%d context items) verified", trendingCount, contextCount)
}

// TestCollector_E2E_IdempotencyCheck verifies that running collection twice
// in the same day is properly prevented.
func TestCollector_E2E_IdempotencyCheck(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "trending_items", "context_items", "collection_runs")

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set - skipping E2E idempotency test")
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash-lite"
	}

	aiClient := gemini.NewClient(apiKey, model)
	trendsCollector := googletrends.NewCollector()
	newsCollector := googlenews.NewCollector(googlenews.DefaultTopics())
	aggregator := collector.NewAggregator([]collector.SourceCollector{trendsCollector, newsCollector})

	repo := storage.NewCollectorRepository(db.Pool)
	trendingRepo := storage.NewTrendingItemRepository(db.Pool)
	service := collector.NewService(aiClient, repo)
	service.WithAggregator(aggregator).WithTrendingRepo(trendingRepo)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// First collection should succeed
	t.Log("First collection attempt...")
	result1, err := service.CollectFromSourcesIfNeeded(ctx)
	if err != nil {
		t.Fatalf("First CollectFromSourcesIfNeeded failed: %v", err)
	}
	if result1 == nil {
		t.Fatal("First collection should return result")
	}
	t.Logf("First collection succeeded: %d items", len(result1.Items))

	// Second collection should be skipped
	t.Log("Second collection attempt (should be skipped)...")
	result2, err := service.CollectFromSourcesIfNeeded(ctx)
	if err != nil {
		t.Fatalf("Second CollectFromSourcesIfNeeded failed: %v", err)
	}
	if result2 != nil {
		t.Error("Second collection should return nil (already collected today)")
	}
	t.Log("Second collection properly skipped (idempotency working)")

	// Verify only one run exists in DB
	var runCount int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM collection_runs").Scan(&runCount)
	if err != nil {
		t.Fatalf("Failed to count runs: %v", err)
	}
	if runCount != 1 {
		t.Errorf("Expected exactly 1 run in DB, got %d", runCount)
	}

	t.Log("Idempotency verified: Only 1 run in database")
}
