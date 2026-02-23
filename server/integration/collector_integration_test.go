package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"ota/domain/collector"
	"ota/storage"
)

type mockAIClient struct {
	mu        sync.Mutex
	responses []collector.AIResponse
	errs      []error
	callIdx   int
}

func (m *mockAIClient) SearchAndAnalyze(_ context.Context, _ string) (collector.AIResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := m.callIdx
	m.callIdx++

	var err error
	if idx < len(m.errs) {
		err = m.errs[idx]
	}
	if err != nil {
		return collector.AIResponse{}, err
	}

	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return collector.AIResponse{}, nil
}

// mockSourceCollector implements collector.SourceCollector for integration tests.
type mockSourceCollector struct {
	name  string
	items []collector.TrendingItem
}

func (m *mockSourceCollector) Name() string { return m.name }
func (m *mockSourceCollector) Collect(_ context.Context) ([]collector.TrendingItem, error) {
	return m.items, nil
}

var testTrendingItems = []collector.TrendingItem{
	{Keyword: "테스트 주제 1", Source: "google_trends", Traffic: 5000, ArticleURLs: []string{"https://example1.com"}, ArticleTitles: []string{"테스트 기사 1"}},
	{Keyword: "테스트 주제 2", Source: "google_news", Category: "entertainment", ArticleURLs: []string{"https://example2.com"}, ArticleTitles: []string{"테스트 기사 2"}},
	{Keyword: "테스트 주제 3", Source: "google_news", Category: "economy", ArticleURLs: []string{"https://example3.com"}, ArticleTitles: []string{"테스트 기사 3"}},
}

func newIntegrationService(aiClient *mockAIClient, pool interface{ Close() }, db *TestDB) *collector.Service {
	repo := storage.NewCollectorRepository(db.Pool)
	sc := &mockSourceCollector{name: "test_source", items: testTrendingItems}
	agg := collector.NewAggregator([]collector.SourceCollector{sc})
	trendingRepo := storage.NewTrendingItemRepository(db.Pool)

	svc := collector.NewService(aiClient, repo)
	svc.WithAggregator(agg).WithTrendingRepo(trendingRepo)
	return svc
}

func TestCollector_FullFlow(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "trending_items", "context_items", "collection_runs")

	aiClient := &mockAIClient{
		responses: []collector.AIResponse{
			{
				OutputText: validJSON,
				RawJSON:    `{"raw":"data"}`,
			},
		},
	}

	service := newIntegrationService(aiClient, nil, db)

	result, err := service.CollectFromSources(context.Background())
	if err != nil {
		t.Fatalf("CollectFromSources failed: %v", err)
	}

	if result.Run.Status != collector.RunStatusSuccess {
		t.Errorf("expected status success, got %s", result.Run.Status)
	}

	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}

	// Verify context items were saved to DB
	rows, err := db.Pool.Query(context.Background(), "SELECT category, topic FROM context_items ORDER BY rank")
	if err != nil {
		t.Fatalf("failed to query items: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var category, topic string
		if err := rows.Scan(&category, &topic); err != nil {
			t.Fatalf("failed to scan row: %v", err)
		}
		count++
	}

	if count != 3 {
		t.Errorf("expected 3 items in DB, got %d", count)
	}

	// Verify trending items were saved to DB
	var trendingCount int
	err = db.Pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM trending_items WHERE collection_run_id = $1", result.Run.ID).Scan(&trendingCount)
	if err != nil {
		t.Fatalf("failed to count trending items: %v", err)
	}
	if trendingCount != 3 {
		t.Errorf("expected 3 trending items in DB, got %d", trendingCount)
	}
}

func TestCollector_CanRunToday(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "trending_items", "context_items", "collection_runs")

	repo := storage.NewCollectorRepository(db.Pool)

	// Initially can run
	canRun, err := repo.CanRunToday(context.Background())
	if err != nil {
		t.Fatalf("CanRunToday failed: %v", err)
	}
	if !canRun {
		t.Error("expected can run initially")
	}

	// Create a running run
	run := collector.CollectionRun{
		ID:        mustUUID(),
		StartedAt: time.Now().UTC(),
		Status:    collector.RunStatusRunning,
	}
	if err := repo.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}

	// Now cannot run (running status blocks)
	canRun, err = repo.CanRunToday(context.Background())
	if err != nil {
		t.Fatalf("CanRunToday failed: %v", err)
	}
	if canRun {
		t.Error("expected cannot run when run is in progress")
	}

	// Complete as failed
	failMsg := "test failure"
	if err := repo.CompleteRun(context.Background(), run.ID, collector.RunStatusFailed, &failMsg, nil); err != nil {
		t.Fatalf("CompleteRun failed: %v", err)
	}

	// Now can run again (failed status doesn't block)
	canRun, err = repo.CanRunToday(context.Background())
	if err != nil {
		t.Fatalf("CanRunToday failed: %v", err)
	}
	if !canRun {
		t.Error("expected can run after failed status")
	}

	// Create successful run
	successRun := collector.CollectionRun{
		ID:        mustUUID(),
		StartedAt: time.Now().UTC(),
		Status:    collector.RunStatusRunning,
	}
	if err := repo.CreateRun(context.Background(), successRun); err != nil {
		t.Fatalf("CreateRun failed: %v", err)
	}
	if err := repo.CompleteRun(context.Background(), successRun.ID, collector.RunStatusSuccess, nil, nil); err != nil {
		t.Fatalf("CompleteRun failed: %v", err)
	}

	// Now cannot run (success status blocks)
	canRun, err = repo.CanRunToday(context.Background())
	if err != nil {
		t.Fatalf("CanRunToday failed: %v", err)
	}
	if canRun {
		t.Error("expected cannot run when run succeeded today")
	}
}

func TestCollector_CollectFromSourcesIfNeeded(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "trending_items", "context_items", "collection_runs")

	aiClient := &mockAIClient{
		responses: []collector.AIResponse{
			{OutputText: validJSON, RawJSON: `{"raw":"data"}`},
		},
	}

	service := newIntegrationService(aiClient, nil, db)

	// First call should succeed
	result1, err := service.CollectFromSourcesIfNeeded(context.Background())
	if err != nil {
		t.Fatalf("CollectFromSourcesIfNeeded failed: %v", err)
	}
	if result1 == nil {
		t.Fatal("expected result on first call")
	}

	// Second call should return nil (already collected today)
	result2, err := service.CollectFromSourcesIfNeeded(context.Background())
	if err != nil {
		t.Fatalf("CollectFromSourcesIfNeeded failed: %v", err)
	}
	if result2 != nil {
		t.Error("expected nil result on second call (already collected)")
	}
}

func TestCollector_MultipleInstances(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "trending_items", "context_items", "collection_runs")

	aiClient := &mockAIClient{
		responses: []collector.AIResponse{
			{OutputText: validJSON, RawJSON: `{"raw":"data"}`},
		},
	}

	service := newIntegrationService(aiClient, nil, db)

	// Simulate two instances calling at the same time
	results := make(chan *collector.CollectionResult, 2)
	errors := make(chan error, 2)

	for i := 0; i < 2; i++ {
		go func() {
			result, err := service.CollectFromSourcesIfNeeded(context.Background())
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}()
	}

	// Collect results
	successCount := 0
	skipCount := 0

	for i := 0; i < 2; i++ {
		select {
		case err := <-errors:
			t.Fatalf("unexpected error: %v", err)
		case result := <-results:
			if result != nil {
				successCount++
			} else {
				skipCount++
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for results")
		}
	}

	// Exactly one should succeed, one should skip
	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount)
	}
	if skipCount != 1 {
		t.Errorf("expected exactly 1 skip, got %d", skipCount)
	}
}

const validJSON = `{
	"items": [
		{
			"category": "top",
			"rank": 1,
			"topic": "테스트 주제 1",
			"summary": "첫 번째 테스트 요약",
			"detail": "첫 번째 테스트 상세 설명입니다.",
			"sources": ["https://example1.com"]
		},
		{
			"category": "entertainment",
			"rank": 1,
			"topic": "테스트 주제 2",
			"summary": "두 번째 테스트 요약",
			"detail": "두 번째 테스트 상세 설명입니다.",
			"sources": ["https://example2.com"]
		},
		{
			"category": "economy",
			"rank": 1,
			"topic": "테스트 주제 3",
			"summary": "세 번째 테스트 요약",
			"detail": "세 번째 테스트 상세 설명입니다.",
			"sources": ["https://example3.com"]
		}
	]
}`

func mustUUID() uuid.UUID {
	return uuid.New()
}
