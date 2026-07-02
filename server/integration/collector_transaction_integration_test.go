package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/domain/collector"
	"ota/storage"
)

func TestCollectorTransaction_AtomicityAndRollback(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	ctx := context.Background()
	repo := storage.NewCollectorRepository(db.Pool)

	// 1. Create a running run
	runID := uuid.New()
	if err := repo.CreateRun(ctx, collector.CollectionRun{
		ID:        runID,
		StartedAt: time.Now().UTC(),
		Status:    collector.RunStatusRunning,
	}); err != nil {
		t.Fatalf("failed to create run: %v", err)
	}

	// 2. Prepare test items
	items := []collector.ContextItem{
		{
			ID:              uuid.New(),
			CollectionRunID: runID,
			Category:        "general",
			Rank:            1,
			Topic:           "Topic A",
			Summary:         "Summary A",
			Detail:          "Detail A",
			BuzzScore:       100,
			Sources:         []string{"https://a.com"},
		},
		{
			ID:              uuid.New(),
			CollectionRunID: runID,
			Category:        "technology",
			Rank:            2,
			Topic:           "Topic B",
			Summary:         "Summary B",
			Detail:          "Detail B",
			BuzzScore:       90,
			Sources:         []string{"https://b.com"},
		},
	}

	// 3. Test Failure scenario: invalid status or foreign key violation
	// Using a non-existent run ID for items triggers foreign key constraint violation.
	badRunID := uuid.New()
	badItems := []collector.ContextItem{
		{
			ID:              uuid.New(),
			CollectionRunID: badRunID, // Non-existent run ID -> triggers FK violation
			Category:        "general",
			Rank:            1,
			Topic:           "Bad Topic",
			Summary:         "Summary",
			Detail:          "Detail",
			Sources:         []string{"https://x.com"},
		},
	}

	err := repo.SaveItemsAndCompleteRun(ctx, badItems, runID, collector.RunStatusSuccess, nil, nil)
	if err == nil {
		t.Fatal("expected error due to foreign key violation, but got nil")
	}

	// Verify that the runID's status is still "running" and NO items were saved (rollback verified)
	var runStatus string
	err = db.Pool.QueryRow(ctx, "SELECT status FROM collection_runs WHERE id = $1", runID).Scan(&runStatus)
	if err != nil {
		t.Fatalf("failed to query run status: %v", err)
	}
	if runStatus != "running" {
		t.Errorf("expected run status to stay 'running', got %s", runStatus)
	}

	var itemCount int
	err = db.Pool.QueryRow(ctx, "SELECT count(*) FROM context_items WHERE collection_run_id = $1", runID).Scan(&itemCount)
	if err != nil {
		t.Fatalf("failed to query context items count: %v", err)
	}
	if itemCount > 0 {
		t.Errorf("expected 0 items saved due to rollback, got %d", itemCount)
	}

	// 4. Test Success scenario
	err = repo.SaveItemsAndCompleteRun(ctx, items, runID, collector.RunStatusSuccess, nil, nil)
	if err != nil {
		t.Fatalf("expected successful transaction, got error: %v", err)
	}

	// Verify that the status changed to "success" and items are stored
	err = db.Pool.QueryRow(ctx, "SELECT status FROM collection_runs WHERE id = $1", runID).Scan(&runStatus)
	if err != nil {
		t.Fatalf("failed to query run status: %v", err)
	}
	if runStatus != "success" {
		t.Errorf("expected run status to be 'success', got %s", runStatus)
	}

	err = db.Pool.QueryRow(ctx, "SELECT count(*) FROM context_items WHERE collection_run_id = $1", runID).Scan(&itemCount)
	if err != nil {
		t.Fatalf("failed to query context items count: %v", err)
	}
	if itemCount != 2 {
		t.Errorf("expected 2 items saved, got %d", itemCount)
	}
}
