package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	"ota/domain/collector"
	"ota/platform/gemini"
	"ota/storage"
)

// TestCollector_E2E_WithRealAPI tests the complete flow:
// 1. Real Gemini API call with web search
// 2. Parse AI response into context items
// 3. Store in PostgreSQL database
// 4. Verify data integrity
func TestCollector_E2E_WithRealAPI(t *testing.T) {
	// Setup test database
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	// Get Gemini credentials from environment
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set - skipping E2E test with real API")
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash-lite"
	}

	t.Logf("Testing E2E flow with real Gemini API (model: %s)", model)

	// Create real Gemini client
	aiClient := gemini.NewClient(apiKey, model)

	// Create repository and service
	repo := storage.NewCollectorRepository(db.Pool)
	service := collector.NewService(aiClient, repo)

	// Execute collection
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	t.Log("Calling collector.Collect() with real AI...")
	result, err := service.Collect(ctx)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
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

	t.Logf("✅ Collection succeeded: %d context items", len(result.Items))

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
		t.Logf("  [%s] Rank %d: %s", item.Category, item.Rank, item.Topic)
	}

	// Verify at least one "top" category exists
	if !categorySeen["top"] {
		t.Error("Expected at least one item with category 'top'")
	}

	// Verify data was stored in database
	t.Log("Verifying data in database...")

	// Check collection_runs table
	var dbRunID, dbStatus string
	var dbStartedAt, dbCompletedAt time.Time
	var dbRawResponse *string
	err = db.Pool.QueryRow(ctx,
		`SELECT id, started_at, completed_at, status, raw_response
		 FROM collection_runs
		 WHERE id = $1`,
		result.Run.ID,
	).Scan(&dbRunID, &dbStartedAt, &dbCompletedAt, &dbStatus, &dbRawResponse)
	if err != nil {
		t.Fatalf("Failed to query collection_runs: %v", err)
	}

	if dbStatus != string(collector.RunStatusSuccess) {
		t.Errorf("DB status mismatch: expected %s, got %s", collector.RunStatusSuccess, dbStatus)
	}

	if dbRawResponse == nil || *dbRawResponse == "" {
		t.Error("raw_response should be stored in DB")
	} else {
		t.Logf("  Raw response stored: %d bytes", len(*dbRawResponse))
	}

	// Check context_items table
	rows, err := db.Pool.Query(ctx,
		`SELECT id, category, rank, topic, summary, sources
		 FROM context_items
		 WHERE collection_run_id = $1
		 ORDER BY category, rank`,
		result.Run.ID,
	)
	if err != nil {
		t.Fatalf("Failed to query context_items: %v", err)
	}
	defer rows.Close()

	dbItemCount := 0
	for rows.Next() {
		var id, category, topic, summary string
		var rank int
		var sources []byte // JSONB

		err := rows.Scan(&id, &category, &rank, &topic, &summary, &sources)
		if err != nil {
			t.Fatalf("Failed to scan context_item: %v", err)
		}

		dbItemCount++
		t.Logf("  DB Item %d: [%s] Rank %d - %s", dbItemCount, category, rank, topic)
	}

	if dbItemCount != len(result.Items) {
		t.Errorf("DB item count mismatch: expected %d, got %d", len(result.Items), dbItemCount)
	}

	t.Logf("✅ E2E test passed: Real API → Parsing → DB Storage verified")
}

// TestCollector_E2E_IdempotencyCheck verifies that running collection twice
// in the same day is properly prevented
func TestCollector_E2E_IdempotencyCheck(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set - skipping E2E idempotency test")
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash-lite"
	}

	aiClient := gemini.NewClient(apiKey, model)
	repo := storage.NewCollectorRepository(db.Pool)
	service := collector.NewService(aiClient, repo)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// First collection should succeed
	t.Log("First collection attempt...")
	result1, err := service.CollectIfNeeded(ctx)
	if err != nil {
		t.Fatalf("First CollectIfNeeded failed: %v", err)
	}
	if result1 == nil {
		t.Fatal("First collection should return result")
	}
	t.Logf("✅ First collection succeeded: %d items", len(result1.Items))

	// Second collection should be skipped
	t.Log("Second collection attempt (should be skipped)...")
	result2, err := service.CollectIfNeeded(ctx)
	if err != nil {
		t.Fatalf("Second CollectIfNeeded failed: %v", err)
	}
	if result2 != nil {
		t.Error("Second collection should return nil (already collected today)")
	}
	t.Log("✅ Second collection properly skipped (idempotency working)")

	// Verify only one run exists in DB
	var runCount int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM collection_runs").Scan(&runCount)
	if err != nil {
		t.Fatalf("Failed to count runs: %v", err)
	}
	if runCount != 1 {
		t.Errorf("Expected exactly 1 run in DB, got %d", runCount)
	}

	t.Log("✅ Idempotency verified: Only 1 run in database")
}
