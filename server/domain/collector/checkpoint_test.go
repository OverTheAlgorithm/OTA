package collector

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- checkpoint mock ---

type mockCheckpointRepo struct {
	mu sync.Mutex

	// SaveCheckpoint
	savedRunID uuid.UUID
	savedStage int
	savedData  json.RawMessage
	saveErr    error

	// GetLatestResumableRun
	resumeRun   *CollectionRun
	resumeStage *int
	resumeData  json.RawMessage
	resumeErr   error

	// ClearCheckpoint
	clearedRunID uuid.UUID
	clearErr     error

	// CreateRunIfIdle
	createOK  bool
	createErr error

	// CleanupOldCheckpoints
	cleanupCount int
	cleanupErr   error
}

func (m *mockCheckpointRepo) SaveCheckpoint(_ context.Context, runID uuid.UUID, stage int, data json.RawMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.savedRunID = runID
	m.savedStage = stage
	m.savedData = data
	return m.saveErr
}

func (m *mockCheckpointRepo) GetLatestResumableRun(_ context.Context, _ time.Duration) (*CollectionRun, *int, json.RawMessage, error) {
	return m.resumeRun, m.resumeStage, m.resumeData, m.resumeErr
}

func (m *mockCheckpointRepo) ClearCheckpoint(_ context.Context, runID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearedRunID = runID
	return m.clearErr
}

func (m *mockCheckpointRepo) CreateRunIfIdle(_ context.Context, _ CollectionRun) (bool, error) {
	return m.createOK, m.createErr
}

func (m *mockCheckpointRepo) CleanupOldCheckpoints(_ context.Context) (int, error) {
	return m.cleanupCount, m.cleanupErr
}

// --- helpers ---

func newTestServiceWithCheckpoint(aiClient AIClient, repo *mockRepo, sc SourceCollector, cpRepo CheckpointRepository) *Service {
	agg := NewAggregator(sc, sc)
	svc := NewService(aiClient, repo, agg, &mockTrendingRepo{}, &mockBrainCatRepo{}, noopURLDecoder, noopArticleFetcher, noopImageGen())
	svc.WithCheckpointRepo(cpRepo)
	return svc
}

func makeCheckpointData(t *testing.T, stage int, payload any) json.RawMessage {
	t.Helper()
	data, err := marshalCheckpoint(stage, payload)
	if err != nil {
		t.Fatalf("marshalCheckpoint: %v", err)
	}
	return data
}

func intPtr(v int) *int { return &v }

// --- tests: checkpoint data round-trip ---

func TestCheckpointRoundTrip_Stage0(t *testing.T) {
	original := Stage0Data{FormattedText: "trending: AI, blockchain, 엔비디아"}
	data, err := marshalCheckpoint(0, original)
	if err != nil {
		t.Fatalf("marshalCheckpoint: %v", err)
	}

	stage, inner, err := unmarshalCheckpoint(data)
	if err != nil {
		t.Fatalf("unmarshalCheckpoint: %v", err)
	}
	if stage != 0 {
		t.Errorf("expected stage 0, got %d", stage)
	}

	var decoded Stage0Data
	if err := json.Unmarshal(inner, &decoded); err != nil {
		t.Fatalf("unmarshal Stage0Data: %v", err)
	}
	if decoded.FormattedText != original.FormattedText {
		t.Errorf("FormattedText mismatch: got %q, want %q", decoded.FormattedText, original.FormattedText)
	}
}

func TestCheckpointRoundTrip_Stage1(t *testing.T) {
	original := Stage1Data{
		Topics: []Phase1Topic{
			{TopicHint: "RTX 5090", Category: "tech", BuzzScore: 85, Sources: []string{"https://example.com"}},
		},
		Phase1RawJSON: `{"raw":"data"}`,
	}
	data, err := marshalCheckpoint(1, original)
	if err != nil {
		t.Fatalf("marshalCheckpoint: %v", err)
	}

	stage, inner, err := unmarshalCheckpoint(data)
	if err != nil {
		t.Fatalf("unmarshalCheckpoint: %v", err)
	}
	if stage != 1 {
		t.Errorf("expected stage 1, got %d", stage)
	}

	var decoded Stage1Data
	if err := json.Unmarshal(inner, &decoded); err != nil {
		t.Fatalf("unmarshal Stage1Data: %v", err)
	}
	if len(decoded.Topics) != 1 || decoded.Topics[0].TopicHint != "RTX 5090" {
		t.Errorf("topics mismatch: got %+v", decoded.Topics)
	}
	if decoded.Phase1RawJSON != original.Phase1RawJSON {
		t.Errorf("Phase1RawJSON mismatch")
	}
}

func TestCheckpointRoundTrip_Stage3(t *testing.T) {
	original := Stage3Data{
		Topics: []Phase1Topic{
			{TopicHint: "뉴진스", Category: "entertainment", BuzzScore: 70, Sources: []string{"https://example.com/news"}},
		},
		ArticleMap: map[int][]FetchedArticle{
			0: {{URL: "https://example.com/news", Body: "article body"}},
		},
		Phase1RawJSON: `{"phase1":"data"}`,
	}
	data, err := marshalCheckpoint(3, original)
	if err != nil {
		t.Fatalf("marshalCheckpoint: %v", err)
	}

	stage, inner, err := unmarshalCheckpoint(data)
	if err != nil {
		t.Fatalf("unmarshalCheckpoint: %v", err)
	}
	if stage != 3 {
		t.Errorf("expected stage 3, got %d", stage)
	}

	var decoded Stage3Data
	if err := json.Unmarshal(inner, &decoded); err != nil {
		t.Fatalf("unmarshal Stage3Data: %v", err)
	}
	if len(decoded.Topics) != 1 {
		t.Errorf("expected 1 topic, got %d", len(decoded.Topics))
	}
	if len(decoded.ArticleMap[0]) != 1 {
		t.Errorf("expected 1 article for index 0, got %d", len(decoded.ArticleMap[0]))
	}
}

func TestCheckpointVersionMismatch(t *testing.T) {
	// Create a checkpoint with a wrong version.
	raw, _ := json.Marshal(StageCheckpoint{Version: 999, Stage: 0, Data: json.RawMessage(`{}`)})
	_, _, err := unmarshalCheckpoint(raw)
	if err == nil {
		t.Fatal("expected error for version mismatch")
	}
	var verErr *CheckpointVersionError
	if !errors.As(err, &verErr) {
		t.Errorf("expected CheckpointVersionError, got %T: %v", err, err)
	}
}

func TestCheckpointCorruptJSON(t *testing.T) {
	_, _, err := unmarshalCheckpoint(json.RawMessage(`not valid json`))
	if err == nil {
		t.Fatal("expected error for corrupt JSON")
	}
}

// --- tests: ResumeOrCollect ---

func TestResumeOrCollect_NilCheckpointRepo_DelegatesToFresh(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{OutputText: validPhase1JSON, RawJSON: `{"phase1":"data"}`},
			{OutputText: validPhase2JSON("RTX 5090"), RawJSON: `{"p2":"1"}`},
			{OutputText: validPhase2JSON("뉴진스"), RawJSON: `{"p2":"2"}`},
		},
	}
	sc := &mockSourceCollector{name: "test", items: testTrendingItems}

	svc := newTestService(aiClient, repo, sc) // no checkpoint repo
	result, err := svc.ResumeOrCollect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}
}

func TestResumeOrCollect_NoResumableRun_DelegatesToFresh(t *testing.T) {
	repo := &mockRepo{}
	cpRepo := &mockCheckpointRepo{
		resumeRun: nil, // no resumable run
		createOK:  true,
	}
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{OutputText: validPhase1JSON, RawJSON: `{"phase1":"data"}`},
			{OutputText: validPhase2JSON("RTX 5090"), RawJSON: `{"p2":"1"}`},
			{OutputText: validPhase2JSON("뉴진스"), RawJSON: `{"p2":"2"}`},
		},
	}
	sc := &mockSourceCollector{name: "test", items: testTrendingItems}

	svc := newTestServiceWithCheckpoint(aiClient, repo, sc, cpRepo)
	result, err := svc.ResumeOrCollect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}
}

func TestResumeOrCollect_ResumeFromStage0(t *testing.T) {
	repo := &mockRepo{}
	// Stage 0 checkpoint: only formatted text saved. Resume should run Stage 1+.
	// AI client needs: Phase 1 response + 2 Phase 2 responses.
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{OutputText: validPhase1JSON, RawJSON: `{"phase1":"data"}`},
			{OutputText: validPhase2JSON("RTX 5090"), RawJSON: `{"p2":"1"}`},
			{OutputText: validPhase2JSON("뉴진스"), RawJSON: `{"p2":"2"}`},
		},
	}

	cpData := makeCheckpointData(t, 0, Stage0Data{FormattedText: "trending items text"})
	oldRunID := uuid.New()
	cpRepo := &mockCheckpointRepo{
		resumeRun:   &CollectionRun{ID: oldRunID, Status: RunStatusFailed},
		resumeStage: intPtr(0),
		resumeData:  cpData,
		createOK:    true,
	}

	sc := &mockSourceCollector{name: "test", items: testTrendingItems}
	svc := newTestServiceWithCheckpoint(aiClient, repo, sc, cpRepo)

	result, err := svc.ResumeOrCollect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items from resumed run, got %d", len(result.Items))
	}
	if result.Run.Status != RunStatusSuccess {
		t.Errorf("expected success status, got %s", result.Run.Status)
	}
	// The run ID should be NEW (not the old failed run's ID).
	if result.Run.ID == oldRunID {
		t.Error("resumed run should have a new ID, not reuse the old failed run's ID")
	}
}

func TestResumeOrCollect_ResumeFromStage1(t *testing.T) {
	repo := &mockRepo{}
	// Stage 1 checkpoint: topics already clustered. Resume should skip Stage 0+1, run Stage 2+3+4+5.
	// AI client only needs Phase 2 responses (no Phase 1).
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{OutputText: validPhase2JSON("RTX 5090"), RawJSON: `{"p2":"1"}`},
			{OutputText: validPhase2JSON("뉴진스"), RawJSON: `{"p2":"2"}`},
		},
	}

	topics := []Phase1Topic{
		{TopicHint: "RTX 5090", Category: "top", BuzzScore: 85, Sources: []string{"https://example.com/news1"}},
		{TopicHint: "뉴진스", Category: "entertainment", BuzzScore: 70, Sources: []string{"https://example.com/news2"}},
	}
	cpData := makeCheckpointData(t, 1, Stage1Data{Topics: topics, Phase1RawJSON: `{"phase1":"cached"}`})
	cpRepo := &mockCheckpointRepo{
		resumeRun:   &CollectionRun{ID: uuid.New(), Status: RunStatusFailed},
		resumeStage: intPtr(1),
		resumeData:  cpData,
		createOK:    true,
	}

	sc := &mockSourceCollector{name: "test", items: testTrendingItems}
	svc := newTestServiceWithCheckpoint(aiClient, repo, sc, cpRepo)

	result, err := svc.ResumeOrCollect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}
	// Verify the AI client was only called for Phase 2 (no Phase 1 call).
	aiClient.mu.Lock()
	callCount := aiClient.callIdx
	aiClient.mu.Unlock()
	if callCount != 2 {
		t.Errorf("expected 2 AI calls (Phase 2 only), got %d", callCount)
	}
}

func TestResumeOrCollect_ResumeFromStage3(t *testing.T) {
	repo := &mockRepo{}
	// Stage 3 checkpoint: topics + articles ready. Resume should only run Stage 4+5+6+7.
	// AI client only needs Phase 2 responses.
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{OutputText: validPhase2JSON("RTX 5090"), RawJSON: `{"p2":"1"}`},
		},
	}

	topics := []Phase1Topic{
		{TopicHint: "RTX 5090", Category: "top", BuzzScore: 85, Sources: []string{"https://example.com/news1"}},
	}
	articleMap := map[int][]FetchedArticle{
		0: {{URL: "https://example.com/news1", Body: "article content"}},
	}
	cpData := makeCheckpointData(t, 3, Stage3Data{Topics: topics, ArticleMap: articleMap, Phase1RawJSON: `{"phase1":"cached"}`})
	cpRepo := &mockCheckpointRepo{
		resumeRun:   &CollectionRun{ID: uuid.New(), Status: RunStatusFailed},
		resumeStage: intPtr(3),
		resumeData:  cpData,
		createOK:    true,
	}

	sc := &mockSourceCollector{name: "test", items: testTrendingItems}
	svc := newTestServiceWithCheckpoint(aiClient, repo, sc, cpRepo)

	result, err := svc.ResumeOrCollect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(result.Items))
	}
	// Verify only 1 AI call (Phase 2 for the single topic).
	aiClient.mu.Lock()
	callCount := aiClient.callIdx
	aiClient.mu.Unlock()
	if callCount != 1 {
		t.Errorf("expected 1 AI call (Phase 2 only), got %d", callCount)
	}
}

func TestResumeOrCollect_CorruptCheckpoint_FallsBackToFresh(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{OutputText: validPhase1JSON, RawJSON: `{"phase1":"data"}`},
			{OutputText: validPhase2JSON("RTX 5090"), RawJSON: `{"p2":"1"}`},
			{OutputText: validPhase2JSON("뉴진스"), RawJSON: `{"p2":"2"}`},
		},
	}

	// Corrupt checkpoint data (invalid JSON envelope).
	cpRepo := &mockCheckpointRepo{
		resumeRun:   &CollectionRun{ID: uuid.New(), Status: RunStatusFailed},
		resumeStage: intPtr(1),
		resumeData:  json.RawMessage(`{corrupted data!!!`),
		createOK:    true,
	}

	sc := &mockSourceCollector{name: "test", items: testTrendingItems}
	svc := newTestServiceWithCheckpoint(aiClient, repo, sc, cpRepo)

	result, err := svc.ResumeOrCollect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to fresh run and produce results.
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items from fresh fallback, got %d", len(result.Items))
	}
}

func TestResumeOrCollect_CreateRunIfIdleBlocked_ReturnsEmpty(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{}

	cpData := makeCheckpointData(t, 3, Stage3Data{
		Topics:        []Phase1Topic{{TopicHint: "test", Category: "top", Sources: []string{"https://example.com"}}},
		ArticleMap:    map[int][]FetchedArticle{0: {{URL: "https://example.com", Body: "body"}}},
		Phase1RawJSON: `{}`,
	})
	cpRepo := &mockCheckpointRepo{
		resumeRun:   &CollectionRun{ID: uuid.New(), Status: RunStatusFailed},
		resumeStage: intPtr(3),
		resumeData:  cpData,
		createOK:    false, // blocked — another run is active
	}

	sc := &mockSourceCollector{name: "test", items: testTrendingItems}
	svc := newTestServiceWithCheckpoint(aiClient, repo, sc, cpRepo)

	result, err := svc.ResumeOrCollect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return empty result (skipped).
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items when blocked, got %d", len(result.Items))
	}
}

func TestResumeOrCollect_VersionMismatch_FallsBackToFresh(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{OutputText: validPhase1JSON, RawJSON: `{"phase1":"data"}`},
			{OutputText: validPhase2JSON("RTX 5090"), RawJSON: `{"p2":"1"}`},
			{OutputText: validPhase2JSON("뉴진스"), RawJSON: `{"p2":"2"}`},
		},
	}

	// Checkpoint with wrong version.
	wrongVersion, _ := json.Marshal(StageCheckpoint{Version: 999, Stage: 1, Data: json.RawMessage(`{}`)})
	cpRepo := &mockCheckpointRepo{
		resumeRun:   &CollectionRun{ID: uuid.New(), Status: RunStatusFailed},
		resumeStage: intPtr(1),
		resumeData:  wrongVersion,
		createOK:    true,
	}

	sc := &mockSourceCollector{name: "test", items: testTrendingItems}
	svc := newTestServiceWithCheckpoint(aiClient, repo, sc, cpRepo)

	result, err := svc.ResumeOrCollect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items from fresh fallback, got %d", len(result.Items))
	}
}

func TestResumeOrCollect_GetResumableRunError_FallsBackToFresh(t *testing.T) {
	repo := &mockRepo{}
	aiClient := &mockAIClient{
		responses: []AIResponse{
			{OutputText: validPhase1JSON, RawJSON: `{"phase1":"data"}`},
			{OutputText: validPhase2JSON("RTX 5090"), RawJSON: `{"p2":"1"}`},
			{OutputText: validPhase2JSON("뉴진스"), RawJSON: `{"p2":"2"}`},
		},
	}

	cpRepo := &mockCheckpointRepo{
		resumeErr: errors.New("db connection lost"),
		createOK:  true,
	}

	sc := &mockSourceCollector{name: "test", items: testTrendingItems}
	svc := newTestServiceWithCheckpoint(aiClient, repo, sc, cpRepo)

	result, err := svc.ResumeOrCollect(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items from fresh fallback, got %d", len(result.Items))
	}
}

// --- tests: saveCheckpoint ---

func TestSaveCheckpoint_NilRepo_NoOp(t *testing.T) {
	svc := &Service{} // no checkpointRepo
	// Should not panic.
	svc.saveCheckpoint(context.Background(), uuid.New(), 0, Stage0Data{FormattedText: "test"})
}

func TestSaveCheckpoint_Error_DoesNotPanic(t *testing.T) {
	cpRepo := &mockCheckpointRepo{saveErr: errors.New("db error")}
	svc := &Service{checkpointRepo: cpRepo}
	// Should log warning but not panic or return error.
	svc.saveCheckpoint(context.Background(), uuid.New(), 0, Stage0Data{FormattedText: "test"})
}

func TestClearCheckpoint_NilRepo_NoOp(t *testing.T) {
	svc := &Service{} // no checkpointRepo
	svc.clearCheckpoint(context.Background(), uuid.New())
}
