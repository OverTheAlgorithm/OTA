package collector

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// --- mocks ---

type mockAIClient struct {
	responses []AIResponse
	errs      []error
	callIdx   int
}

func (m *mockAIClient) SearchAndAnalyze(_ context.Context, _ string) (AIResponse, error) {
	idx := m.callIdx
	m.callIdx++

	var err error
	if idx < len(m.errs) {
		err = m.errs[idx]
	}
	if err != nil {
		return AIResponse{}, err
	}

	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return AIResponse{}, nil
}

type mockRepo struct {
	createRunErr     error
	completeRunErr   error
	saveItemsErr     error
	completedStatus  RunStatus
	completedErrMsg  *string
	completedRawResp *string
	savedItems       []ContextItem
	canRunToday      bool
	canRunTodayErr   error
}

func (m *mockRepo) CreateRun(_ context.Context, _ CollectionRun) error {
	return m.createRunErr
}

func (m *mockRepo) CompleteRun(_ context.Context, _ uuid.UUID, status RunStatus, errMsg *string, rawResp *string) error {
	m.completedStatus = status
	m.completedErrMsg = errMsg
	m.completedRawResp = rawResp
	return m.completeRunErr
}

func (m *mockRepo) SaveContextItems(_ context.Context, items []ContextItem) error {
	m.savedItems = items
	return m.saveItemsErr
}

func (m *mockRepo) CanRunToday(_ context.Context) (bool, error) {
	return m.canRunToday, m.canRunTodayErr
}

// mockSourceCollector is defined in aggregator_test.go (same package).

// mockTrendingRepo records saved items for verification.
type mockTrendingRepo struct {
	savedItems []TrendingItem
	savedRunID uuid.UUID
	saveErr    error
}

func (m *mockTrendingRepo) SaveTrendingItems(_ context.Context, runID uuid.UUID, items []TrendingItem) error {
	m.savedRunID = runID
	m.savedItems = items
	return m.saveErr
}

func (m *mockTrendingRepo) GetTrendingItemsByRunID(_ context.Context, _ uuid.UUID) ([]TrendingItem, error) {
	return m.savedItems, nil
}

// mockBrainCatRepo is a no-op brain category repository for tests.
type mockBrainCatRepo struct{}

func (m *mockBrainCatRepo) GetAll(_ context.Context) ([]BrainCategory, error) { return nil, nil }
func (m *mockBrainCatRepo) Create(_ context.Context, _ BrainCategory) error   { return nil }
func (m *mockBrainCatRepo) Update(_ context.Context, _ BrainCategory) error   { return nil }
func (m *mockBrainCatRepo) Delete(_ context.Context, _ string) error          { return nil }

// noopURLDecoder is a no-op URL decoder for tests.
func noopURLDecoder(_ context.Context, _ ...[]string) int { return 0 }

type noopImageClient struct{}

func (n *noopImageClient) Generate(_ context.Context, _ string) ([]byte, string, error) {
	return nil, "", nil
}

func noopImageGen() *ImageGenerator {
	return NewImageGenerator(&noopImageClient{}, "testdata/images")
}

// --- helpers ---

func newTestService(aiClient AIClient, repo *mockRepo, sc SourceCollector) *Service {
	agg := NewAggregator(sc, sc)
	return NewService(aiClient, repo, agg, &mockTrendingRepo{}, &mockBrainCatRepo{}, noopURLDecoder, noopImageGen())
}

func newTestServiceWithTrendingRepo(aiClient AIClient, repo *mockRepo, sc SourceCollector, trendingRepo TrendingItemRepository) *Service {
	agg := NewAggregator(sc, sc)
	return NewService(aiClient, repo, agg, trendingRepo, &mockBrainCatRepo{}, noopURLDecoder, noopImageGen())
}

var testTrendingItems = []TrendingItem{
	{Keyword: "RTX 5090", Source: "google_trends", Traffic: 5000, ArticleURLs: []string{"https://example.com/rtx"}, ArticleTitles: []string{"RTX 5090 출시"}},
	{Keyword: "엔비디아 신제품 발표", Source: "google_news", Category: "technology", ArticleURLs: []string{"https://example.com/nvidia"}, ArticleTitles: []string{"엔비디아 신제품"}},
}

// --- tests ---

func TestCollectFromSources_Success(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{OutputText: validCollectionJSON, RawJSON: `{"raw":"data"}`},
		},
	}
	collector := &mockSourceCollector{name: "test_source", items: testTrendingItems}

	svc := newTestService(aiClient, repo, collector)
	result, err := svc.CollectFromSources(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Run.Status != RunStatusSuccess {
		t.Errorf("expected status success, got %s", result.Run.Status)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0].Category != "top" {
		t.Errorf("expected category top, got %s", result.Items[0].Category)
	}
	if repo.completedStatus != RunStatusSuccess {
		t.Errorf("expected repo completed with success, got %s", repo.completedStatus)
	}
	if len(repo.savedItems) != 2 {
		t.Errorf("expected 2 saved items, got %d", len(repo.savedItems))
	}
}

func TestCollectFromSources_AIFailure(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{errs: []error{errors.New("ai down")}}
	collector := &mockSourceCollector{name: "test_source", items: testTrendingItems}

	svc := newTestService(aiClient, repo, collector)
	_, err := svc.CollectFromSources(context.Background())
	if err == nil {
		t.Fatal("expected error when AI fails")
	}
	if repo.completedStatus != RunStatusFailed {
		t.Errorf("expected repo completed with failed, got %s", repo.completedStatus)
	}
}

func TestCollectFromSources_MalformedAIResponse(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{
		responses: []AIResponse{{OutputText: "not json at all", RawJSON: `{"raw":"bad"}`}},
	}
	collector := &mockSourceCollector{name: "test_source", items: testTrendingItems}

	svc := newTestService(aiClient, repo, collector)
	_, err := svc.CollectFromSources(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed AI response")
	}
	if repo.completedStatus != RunStatusFailed {
		t.Errorf("expected repo completed with failed, got %s", repo.completedStatus)
	}
	if repo.completedRawResp == nil {
		t.Error("expected raw response to be saved on parse failure")
	}
}

func TestCollectFromSources_CreateRunFailure(t *testing.T) {
	repo := &mockRepo{createRunErr: errors.New("db down")}
	aiClient := &mockAIClient{}
	collector := &mockSourceCollector{name: "test_source", items: testTrendingItems}

	svc := newTestService(aiClient, repo, collector)
	_, err := svc.CollectFromSources(context.Background())
	if err == nil {
		t.Fatal("expected error when CreateRun fails")
	}
}

func TestCollectFromSources_SourceCollectionFails_BothDown(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{}
	badCollector := &mockSourceCollector{name: "bad_source", err: errors.New("rss down")}

	svc := newTestService(aiClient, repo, badCollector)
	_, err := svc.CollectFromSources(context.Background())
	if err == nil {
		t.Fatal("expected error when all sources fail")
	}
	if repo.completedStatus != RunStatusFailed {
		t.Errorf("expected repo completed with failed, got %s", repo.completedStatus)
	}
}


func TestCollectFromSources_SkipsInvalidItems(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{
				OutputText: `{"items":[
				{"category":"top","rank":1,"topic":"유효","summary":"유효한 항목","sources":[]},
				{"category":"","rank":2,"topic":"빈 카테고리","summary":"필터됨","sources":[]},
				{"category":"top","rank":3,"topic":"","summary":"빈 토픽","sources":[]}
			]}`,
				RawJSON: "{}",
			},
		},
	}
	collector := &mockSourceCollector{name: "test_source", items: testTrendingItems}

	svc := newTestService(aiClient, repo, collector)
	result, err := svc.CollectFromSources(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 valid item after filtering, got %d", len(result.Items))
	}
}

func TestCollectFromSources_SavesTrendingItems(t *testing.T) {
	repo := &mockRepo{}
	trendingRepo := &mockTrendingRepo{}
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{OutputText: validCollectionJSON, RawJSON: `{"raw":"data"}`},
		},
	}
	collector := &mockSourceCollector{name: "test_source", items: testTrendingItems}

	svc := newTestServiceWithTrendingRepo(aiClient, repo, collector, trendingRepo)

	_, err := svc.CollectFromSources(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Items are collected from both trends and news (same mock), so 2*2=4
	if len(trendingRepo.savedItems) != 4 {
		t.Errorf("expected 4 trending items saved, got %d", len(trendingRepo.savedItems))
	}
}

func TestCollectFromSourcesIfNeeded_AlreadyRun(t *testing.T) {
	repo := &mockRepo{canRunToday: false}
	aiClient := &mockAIClient{}
	sc := &mockSourceCollector{name: "test_source", items: testTrendingItems}

	svc := newTestService(aiClient, repo, sc)
	result, err := svc.CollectFromSourcesIfNeeded(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result when collection already run")
	}
}

func TestCollectFromSourcesIfNeeded_CanRun(t *testing.T) {
	repo := &mockRepo{canRunToday: true}
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{OutputText: validCollectionJSON, RawJSON: `{"raw":"data"}`},
		},
	}
	collector := &mockSourceCollector{name: "test_source", items: testTrendingItems}

	svc := newTestService(aiClient, repo, collector)
	result, err := svc.CollectFromSourcesIfNeeded(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result when collection can run")
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}
}

const validCollectionJSON = `{
	"items": [
		{
			"category": "top",
			"rank": 1,
			"topic": "RTX 5090 출시",
			"summary": "엔비디아가 RTX 5090을 출시해서 화제예요.",
			"detail": "엔비디아가 차세대 그래픽카드 RTX 5090을 공식 발표했는데요.",
			"sources": ["https://example.com/news1"]
		},
		{
			"category": "entertainment",
			"rank": 1,
			"topic": "뉴진스 컴백",
			"summary": "뉴진스가 새 앨범으로 컴백을 발표했대요.",
			"detail": "뉴진스가 새 앨범 발매를 공식 발표했는데요.",
			"sources": ["https://example.com/news2"]
		}
	]
}`
