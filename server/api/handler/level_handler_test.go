package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ota/api/handler"
	"ota/cache"
	"ota/domain/collector"
	"ota/domain/level"
)

// ─── Mocks ──────────────────────────────────────────────────────────────────

type mockLevelHistoryRepo struct {
	topic      *collector.TopicDetail
	topicErr   error
	isToday    bool
	isTodayErr error
}

func (m *mockLevelHistoryRepo) GetHistoryForUser(_ context.Context, _ string, _, _ int) ([]collector.HistoryEntry, bool, error) {
	return nil, false, nil
}

func (m *mockLevelHistoryRepo) GetContextItemByID(_ context.Context, _ uuid.UUID) (*collector.TopicDetail, error) {
	return m.topic, m.topicErr
}

func (m *mockLevelHistoryRepo) IsRunCreatedToday(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.isToday, m.isTodayErr
}

func (m *mockLevelHistoryRepo) GetRecentTopics(_ context.Context, _ int) ([]collector.TopicPreview, error) {
	return nil, nil
}

func (m *mockLevelHistoryRepo) GetAllTopics(_ context.Context, _, _ string, _, _ int) ([]collector.TopicPreview, bool, error) {
	return nil, false, nil
}

func (m *mockLevelHistoryRepo) GetItemCategoryMap(_ context.Context, _ []uuid.UUID) (map[uuid.UUID]collector.ItemMeta, error) {
	return nil, nil
}

type mockLevelRepo struct {
	coins         int
	earnErr       error
	alreadyEarned bool
	hasEarned     bool
	hasEarnedErr  error
}

func (m *mockLevelRepo) GetUserCoins(_ context.Context, userID string) (level.UserCoins, error) {
	return level.UserCoins{UserID: userID, Coins: m.coins}, nil
}

func (m *mockLevelRepo) EarnCoin(_ context.Context, _ string, _, _ uuid.UUID, coins int) (bool, int, error) {
	if m.earnErr != nil {
		return false, 0, m.earnErr
	}
	if m.alreadyEarned {
		return false, 0, nil
	}
	return true, m.coins + coins, nil
}

func (m *mockLevelRepo) SetCoins(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockLevelRepo) GetTodayEarnedCoins(_ context.Context, _ string) (int, error) {
	return 0, nil
}

func (m *mockLevelRepo) HasEarned(_ context.Context, _ string, _, _ uuid.UUID) (bool, error) {
	return m.hasEarned, m.hasEarnedErr
}

func (m *mockLevelRepo) DeductCoins(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockLevelRepo) RestoreCoins(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockLevelRepo) InsertCoinEvent(_ context.Context, _ string, _ int, _, _, _ string) error {
	return nil
}

func (m *mockLevelRepo) GetCoinHistory(_ context.Context, _ string, _, _ int) ([]level.CoinTransaction, error) {
	return nil, nil
}

func (m *mockLevelRepo) GetEarnedItemIDs(_ context.Context, _ string, _ []uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

type mockSubGetterLevel struct {
	subs    []string
	subsErr error
}

func (m *mockSubGetterLevel) GetSubscriptions(_ context.Context, _ string) ([]string, error) {
	return m.subs, m.subsErr
}

// mockCache is an in-memory cache for tests (no TTL enforcement — just store/fetch).
type mockCache struct {
	store map[string]any
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string]any)}
}

func (c *mockCache) Get(k string) (any, bool) {
	v, ok := c.store[k]
	return v, ok
}

func (c *mockCache) Set(k string, v any, _ time.Duration) {
	c.store[k] = v
}

func (c *mockCache) Delete(k string) {
	delete(c.store, k)
}

func (c *mockCache) Has(k string) bool {
	_, ok := c.store[k]
	return ok
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// fakeAuthMW injects userID into the context, simulating JWT auth middleware.
func fakeAuthMW(userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	}
}

func newLevelTestRouter(
	histRepo collector.HistoryRepository,
	lvlSvc *level.Service,
	sub handler.SubscriptionGetter,
	earnCache cache.Cache,
	earnMinDuration time.Duration,
	userID string,
) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewLevelHandler(lvlSvc, histRepo, sub, earnCache, earnMinDuration, "dummy-secret-key", fakeAuthMW(userID))
	h.RegisterRoutes(r.Group("/level"))
	return r
}

func postEarn(r *gin.Engine, contextItemID, turnstileToken string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{
		"context_item_id": contextItemID,
		"turnstile_token": turnstileToken,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/level/earn", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func postInitEarn(r *gin.Engine, contextItemID string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{
		"context_item_id": contextItemID,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/level/init-earn", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// ─── EarnCoin Tests ──────────────────────────────────────────────────────────

func TestEarnCoin_Success_Preferred(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	histRepo := &mockLevelHistoryRepo{
		topic:   &collector.TopicDetail{ID: topicID, RunID: runID, Category: "sports", Topic: "스포츠"},
		isToday: true,
	}
	svc := level.NewService(&mockLevelRepo{coins: 0}, level.NewLevelConfig(5000, 1000), 0, 0)
	sub := &mockSubGetterLevel{subs: []string{"sports"}}

	mc := newMockCache()
	cacheKey := fmt.Sprintf("earn:%s:%s", userID, topicID)
	mc.Set(cacheKey, handler.EarnPending{
		InitiatedAt:   time.Now().Add(-time.Minute),
		UID:           userID,
		ContextItemID: topicID,
		RunID:         runID,
	}, time.Hour)

	r := newLevelTestRouter(histRepo, svc, sub, mc, 5*time.Second, userID)
	w := postEarn(r, topicID.String(), "dummy-token")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["earned"] != true {
		t.Error("expected earned = true")
	}
	if data["reason"] != "EARNED" {
		t.Errorf("expected reason EARNED, got %v", data["reason"])
	}
}

func TestEarnCoin_TooEarly_NoCache(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	histRepo := &mockLevelHistoryRepo{topic: &collector.TopicDetail{ID: topicID, RunID: runID, Category: "top"}, isToday: true}
	svc := level.NewService(&mockLevelRepo{coins: 0}, level.NewLevelConfig(5000, 1000), 0, 0)

	r := newLevelTestRouter(histRepo, svc, &mockSubGetterLevel{}, newMockCache(), 5*time.Second, userID)
	w := postEarn(r, topicID.String(), "dummy-token")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 TOO_EARLY when cache empty, got %d", w.Code)
	}
}

func TestEarnCoin_Expired(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	histRepo := &mockLevelHistoryRepo{
		topic:   &collector.TopicDetail{ID: topicID, RunID: runID, Category: "top", Topic: "오래된 주제"},
		isToday: false,
	}
	svc := level.NewService(&mockLevelRepo{coins: 0}, level.NewLevelConfig(5000, 1000), 0, 0)

	mc := newMockCache()
	cacheKey := fmt.Sprintf("earn:%s:%s", userID, topicID)
	mc.Set(cacheKey, handler.EarnPending{
		InitiatedAt:   time.Now().Add(-time.Minute),
		UID:           userID,
		ContextItemID: topicID,
		RunID:         runID,
	}, time.Hour)

	r := newLevelTestRouter(histRepo, svc, &mockSubGetterLevel{}, mc, 5*time.Second, userID)
	w := postEarn(r, topicID.String(), "dummy-token")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["reason"] != "EXPIRED" {
		t.Errorf("expected reason EXPIRED, got %v", data["reason"])
	}
}

func TestEarnCoin_Duplicate(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	histRepo := &mockLevelHistoryRepo{
		topic:   &collector.TopicDetail{ID: topicID, RunID: runID, Category: "top", Topic: "이미 읽음"},
		isToday: true,
	}
	svc := level.NewService(&mockLevelRepo{coins: 5, alreadyEarned: true}, level.NewLevelConfig(5000, 1000), 0, 0)

	mc := newMockCache()
	cacheKey := fmt.Sprintf("earn:%s:%s", userID, topicID)
	mc.Set(cacheKey, handler.EarnPending{
		InitiatedAt:   time.Now().Add(-time.Minute),
		UID:           userID,
		ContextItemID: topicID,
		RunID:         runID,
	}, time.Hour)

	r := newLevelTestRouter(histRepo, svc, &mockSubGetterLevel{subs: []string{}}, mc, 5*time.Second, userID)
	w := postEarn(r, topicID.String(), "dummy-token")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["reason"] != "DUPLICATE" {
		t.Errorf("expected reason DUPLICATE, got %v", data["reason"])
	}
}

func TestEarnCoin_InvalidContextItemID(t *testing.T) {
	userID := uuid.New().String()
	svc := level.NewService(&mockLevelRepo{}, level.NewLevelConfig(5000, 1000), 0, 0)
	r := newLevelTestRouter(&mockLevelHistoryRepo{}, svc, &mockSubGetterLevel{}, newMockCache(), 5*time.Second, userID)

	w := postEarn(r, "not-a-uuid", "dummy-token")
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid context_item_id, got %d", w.Code)
	}
}

func TestEarnCoin_MissingFields(t *testing.T) {
	userID := uuid.New().String()
	svc := level.NewService(&mockLevelRepo{}, level.NewLevelConfig(5000, 1000), 0, 0)
	r := newLevelTestRouter(&mockLevelHistoryRepo{}, svc, &mockSubGetterLevel{}, newMockCache(), 5*time.Second, userID)

	body, _ := json.Marshal(map[string]string{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/level/earn", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", w.Code)
	}
}

func TestEarnCoin_InvalidTurnstileToken(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	histRepo := &mockLevelHistoryRepo{
		topic:   &collector.TopicDetail{ID: topicID, RunID: runID, Category: "top"},
		isToday: true,
	}
	svc := level.NewService(&mockLevelRepo{coins: 0}, level.NewLevelConfig(5000, 1000), 0, 0)

	mc := newMockCache()
	cacheKey := fmt.Sprintf("earn:%s:%s", userID, topicID)
	mc.Set(cacheKey, handler.EarnPending{
		InitiatedAt:   time.Now().Add(-time.Minute),
		UID:           userID,
		ContextItemID: topicID,
		RunID:         runID,
	}, time.Hour)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewLevelHandler(svc, histRepo, &mockSubGetterLevel{}, mc, 5*time.Second, "2x0000000000000000000000000000000AA", fakeAuthMW(userID))
	h.RegisterRoutes(r.Group("/level"))

	w := postEarn(r, topicID.String(), "invalid-token-here")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for invalid turnstile token, got %d", w.Code)
	}
}

func TestEarnCoin_ContextItemNotFound(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	histRepo := &mockLevelHistoryRepo{topic: nil, isToday: true}
	svc := level.NewService(&mockLevelRepo{}, level.NewLevelConfig(5000, 1000), 0, 0)

	mc := newMockCache()
	cacheKey := fmt.Sprintf("earn:%s:%s", userID, topicID)
	mc.Set(cacheKey, handler.EarnPending{
		InitiatedAt:   time.Now().Add(-time.Minute),
		UID:           userID,
		ContextItemID: topicID,
		RunID:         runID,
	}, time.Hour)

	r := newLevelTestRouter(histRepo, svc, &mockSubGetterLevel{}, mc, 5*time.Second, userID)
	w := postEarn(r, topicID.String(), "dummy-token")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing context item, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// ─── InitEarn Tests ───────────────────────────────────────────────────────────

func TestInitEarn_Success(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	histRepo := &mockLevelHistoryRepo{
		topic:   &collector.TopicDetail{ID: topicID, RunID: runID, Category: "top", Topic: "테스트"},
		isToday: true,
	}
	svc := level.NewService(&mockLevelRepo{coins: 0}, level.NewLevelConfig(5000, 1000), 0, 0)

	r := newLevelTestRouter(histRepo, svc, &mockSubGetterLevel{}, newMockCache(), 10*time.Second, userID)
	w := postInitEarn(r, topicID.String())

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["status"] != "PENDING" {
		t.Errorf("expected status PENDING, got %v", data["status"])
	}
	if _, ok := data["required_seconds"]; !ok {
		t.Error("expected required_seconds in response")
	}
}

func TestInitEarn_Expired(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	histRepo := &mockLevelHistoryRepo{
		topic:   &collector.TopicDetail{ID: topicID, RunID: runID, Category: "top"},
		isToday: false,
	}
	svc := level.NewService(&mockLevelRepo{}, level.NewLevelConfig(5000, 1000), 0, 0)

	r := newLevelTestRouter(histRepo, svc, &mockSubGetterLevel{}, newMockCache(), 5*time.Second, userID)
	w := postInitEarn(r, topicID.String())

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["status"] != "EXPIRED" {
		t.Errorf("expected EXPIRED, got %v", data["status"])
	}
}

func TestInitEarn_Duplicate(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	histRepo := &mockLevelHistoryRepo{
		topic:   &collector.TopicDetail{ID: topicID, RunID: runID, Category: "top"},
		isToday: true,
	}
	svc := level.NewService(&mockLevelRepo{hasEarned: true}, level.NewLevelConfig(5000, 1000), 0, 0)

	r := newLevelTestRouter(histRepo, svc, &mockSubGetterLevel{}, newMockCache(), 5*time.Second, userID)
	w := postInitEarn(r, topicID.String())

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["status"] != "DUPLICATE" {
		t.Errorf("expected DUPLICATE, got %v", data["status"])
	}
}

func TestInitEarn_TimerReset(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	histRepo := &mockLevelHistoryRepo{
		topic:   &collector.TopicDetail{ID: topicID, RunID: runID, Category: "top"},
		isToday: true,
	}
	svc := level.NewService(&mockLevelRepo{}, level.NewLevelConfig(5000, 1000), 0, 0)
	mc := newMockCache()

	r := newLevelTestRouter(histRepo, svc, &mockSubGetterLevel{}, mc, 5*time.Second, userID)

	w1 := postInitEarn(r, topicID.String())
	if w1.Code != http.StatusOK {
		t.Fatalf("first init-earn: expected 200, got %d", w1.Code)
	}

	w2 := postInitEarn(r, topicID.String())
	if w2.Code != http.StatusOK {
		t.Fatalf("second init-earn: expected 200, got %d", w2.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["status"] != "PENDING" {
		t.Errorf("expected PENDING on second call, got %v", data["status"])
	}
}

func TestInitEarn_MissingFields(t *testing.T) {
	userID := uuid.New().String()
	svc := level.NewService(&mockLevelRepo{}, level.NewLevelConfig(5000, 1000), 0, 0)
	r := newLevelTestRouter(&mockLevelHistoryRepo{}, svc, &mockSubGetterLevel{}, newMockCache(), 5*time.Second, userID)

	body, _ := json.Marshal(map[string]string{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/level/init-earn", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", w.Code)
	}
}

// ─── Verify unused import suppression ─────────────────────────────────────
var _ = fmt.Sprintf // suppress unused import
