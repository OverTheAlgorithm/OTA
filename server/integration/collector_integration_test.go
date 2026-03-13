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

// noopURLDecoder is a no-op URL decoder for tests.
func noopURLDecoder(_ context.Context, _ ...[]string) int { return 0 }

// noopArticleFetcher is a no-op article fetcher for tests.
func noopArticleFetcher(_ context.Context, urls []string) []collector.FetchedArticle {
	result := make([]collector.FetchedArticle, len(urls))
	for i, u := range urls {
		result[i] = collector.FetchedArticle{URL: u, Body: "Test article content"}
	}
	return result
}

// noopImageGen returns a no-op ImageGenerator for tests.
func noopImageGen() *collector.ImageGenerator {
	return collector.NewImageGenerator(&noopImageClient{}, "testdata/images")
}

type noopImageClient struct{}

func (n *noopImageClient) Generate(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", nil
}

func newIntegrationService(aiClient *mockAIClient, _ interface{ Close() }, db *TestDB) *collector.Service {
	repo := storage.NewCollectorRepository(db.Pool)
	sc := &mockSourceCollector{name: "test_source", items: testTrendingItems}
	agg := collector.NewAggregator(sc, sc)
	trendingRepo := storage.NewTrendingItemRepository(db.Pool)
	brainCatRepo := storage.NewBrainCategoryRepository(db.Pool)

	return collector.NewService(aiClient, repo, agg, trendingRepo, brainCatRepo, noopURLDecoder, noopArticleFetcher, noopImageGen())
}

// twoPhaseResponses returns Phase 1 + Phase 2 responses for the 3 test topics.
func twoPhaseResponses() []collector.AIResponse {
	return []collector.AIResponse{
		{OutputText: validPhase1JSON, RawJSON: `{"phase1":"data"}`},
		{OutputText: phase2JSON("테스트 주제 1 제목", "첫 번째 테스트 요약"), RawJSON: `{"phase2":"topic1"}`},
		{OutputText: phase2JSON("테스트 주제 2 제목", "두 번째 테스트 요약"), RawJSON: `{"phase2":"topic2"}`},
		{OutputText: phase2JSON("테스트 주제 3 제목", "세 번째 테스트 요약"), RawJSON: `{"phase2":"topic3"}`},
	}
}

func TestCollector_FullFlow(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "trending_items", "context_items", "collection_runs")

	aiClient := &mockAIClient{
		responses: twoPhaseResponses(),
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
		responses: twoPhaseResponses(),
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

	// Need enough responses for both goroutines (one will succeed with full pipeline)
	aiClient := &mockAIClient{
		responses: append(twoPhaseResponses(), twoPhaseResponses()...),
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

const validPhase1JSON = `{
	"topics": [
		{
			"topic_hint": "테스트 주제 1",
			"category": "top",
			"brain_category": "trend",
			"buzz_score": 85,
			"sources": ["https://example1.com"]
		},
		{
			"topic_hint": "테스트 주제 2",
			"category": "entertainment",
			"brain_category": "fun",
			"buzz_score": 70,
			"sources": ["https://example2.com"]
		},
		{
			"topic_hint": "테스트 주제 3",
			"category": "economy",
			"brain_category": "must_know",
			"buzz_score": 60,
			"sources": ["https://example3.com"]
		}
	]
}`

func phase2JSON(topic, summary string) string {
	return `{
		"topic": "` + topic + `",
		"summary": "` + summary + `",
		"detail": "상세 설명입니다.",
		"details": [{"title": "핵심 포인트", "content": "내용입니다."}]
	}`
}

func mustUUID() uuid.UUID {
	return uuid.New()
}
