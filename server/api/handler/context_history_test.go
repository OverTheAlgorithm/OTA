package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ota/api/handler"
	"ota/domain/collector"
	"ota/domain/level"
	"ota/storage"
)

// ─── Mocks ──────────────────────────────────────────────────────────────────

type mockHistoryRepo struct {
	topic        *collector.TopicDetail
	topicErr     error
	isToday      bool
	isTodayErr   error
	historyErr   error
	historyItems []collector.HistoryEntry
}

func (m *mockHistoryRepo) GetHistoryForUser(_ context.Context, _ string) ([]collector.HistoryEntry, error) {
	return m.historyItems, m.historyErr
}

func (m *mockHistoryRepo) GetContextItemByID(_ context.Context, _ uuid.UUID) (*collector.TopicDetail, error) {
	return m.topic, m.topicErr
}

func (m *mockHistoryRepo) IsRunCreatedToday(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.isToday, m.isTodayErr
}

// mockLevelRepo implements level.Repository for level.Service.
type mockLevelRepo struct {
	points    int
	earnErr   error
	alreadyEarned bool
}

func (m *mockLevelRepo) GetUserPoints(_ context.Context, userID string) (level.UserPoints, error) {
	return level.UserPoints{UserID: userID, Points: m.points}, nil
}

func (m *mockLevelRepo) EarnPoint(_ context.Context, _ string, _, _ uuid.UUID, pts int) (bool, int, error) {
	if m.earnErr != nil {
		return false, 0, m.earnErr
	}
	if m.alreadyEarned {
		return false, 0, nil
	}
	return true, m.points + pts, nil
}

func (m *mockLevelRepo) GetLastEarnedAt(_ context.Context, _ string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

func (m *mockLevelRepo) GetLastEarnedAtBatch(_ context.Context, _ []string) (map[string]time.Time, error) {
	return nil, nil
}

func (m *mockLevelRepo) DecayPoints(_ context.Context, _ int) (int, error) {
	return 0, nil
}

func (m *mockLevelRepo) SetPoints(_ context.Context, _ string, _ int) error {
	return nil
}

func (m *mockLevelRepo) CreateMockOTAItem(_ context.Context) (uuid.UUID, error) {
	return uuid.New(), nil
}

type mockSubGetter struct {
	subs    []string
	subsErr error
}

func (m *mockSubGetter) GetSubscriptions(_ context.Context, _ string) ([]string, error) {
	return m.subs, m.subsErr
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newTestRouter(repo collector.HistoryRepository, lvlSvc *level.Service, sub handler.SubscriptionGetter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewContextHistoryHandler(repo, lvlSvc, sub, func(c *gin.Context) { c.Next() })
	h.RegisterRoutes(r.Group("/context"))
	return r
}

// ─── Tests for GetTopicByID ───────────────────────────────────────────────────

// TestGetTopicByID_NotFound: 존재하지 않는 topic 요청 시 404 반환
func TestGetTopicByID_NotFound(t *testing.T) {
	repo := &mockHistoryRepo{topic: nil}
	svc := level.NewService(&mockLevelRepo{})
	r := newTestRouter(repo, svc, &mockSubGetter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/context/topic/%s", uuid.New()), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestGetTopicByID_InvalidID: UUID 포맷이 아닌 id를 보내면 400 반환
func TestGetTopicByID_InvalidID(t *testing.T) {
	repo := &mockHistoryRepo{}
	svc := level.NewService(&mockLevelRepo{})
	r := newTestRouter(repo, svc, &mockSubGetter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/context/topic/not-a-uuid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestGetTopicByID_RepoError: repo 오류 시 500 반환
func TestGetTopicByID_RepoError(t *testing.T) {
	repo := &mockHistoryRepo{topicErr: errors.New("db error")}
	svc := level.NewService(&mockLevelRepo{})
	r := newTestRouter(repo, svc, &mockSubGetter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/context/topic/%s", uuid.New()), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestGetTopicByID_NoRidUID: rid/uid 없이 조회하면 포인트 로직 없이 200 반환
func TestGetTopicByID_NoRidUID(t *testing.T) {
	topicID := uuid.New()
	repo := &mockHistoryRepo{
		topic: &collector.TopicDetail{
			ID:       topicID,
			Category: "top",
			Topic:    "테스트 주제",
		},
	}
	svc := level.NewService(&mockLevelRepo{})
	r := newTestRouter(repo, svc, &mockSubGetter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/context/topic/%s", topicID), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestGetTopicByID_EarnPoint_PreferredCategory: 선호 카테고리 클릭 시 포인트 적립 (200)
func TestGetTopicByID_EarnPoint_PreferredCategory(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	repo := &mockHistoryRepo{
		topic: &collector.TopicDetail{
			ID:       topicID,
			Category: "sports", // user subscribes to sports → preferred
			Topic:    "스포츠 뉴스",
		},
		isToday: true,
	}
	levelRepo := &mockLevelRepo{points: 0}
	svc := level.NewService(levelRepo)
	sub := &mockSubGetter{subs: []string{"sports"}}

	r := newTestRouter(repo, svc, sub)

	w := httptest.NewRecorder()
	url := fmt.Sprintf("/context/topic/%s?rid=%s&uid=%s", topicID, runID, userID)
	req, _ := http.NewRequest("GET", url, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestGetTopicByID_EarnPoint_NonPreferredCategory: 비선호 카테고리 클릭 시 더 많은 포인트 경로 (200)
func TestGetTopicByID_EarnPoint_NonPreferredCategory(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	repo := &mockHistoryRepo{
		topic: &collector.TopicDetail{
			ID:       topicID,
			Category: "economy", // user doesn't subscribe → non-preferred
			Topic:    "경제 뉴스",
		},
		isToday: true,
	}
	levelRepo := &mockLevelRepo{points: 0}
	svc := level.NewService(levelRepo)
	sub := &mockSubGetter{subs: []string{"sports"}}

	r := newTestRouter(repo, svc, sub)

	w := httptest.NewRecorder()
	url := fmt.Sprintf("/context/topic/%s?rid=%s&uid=%s", topicID, runID, userID)
	req, _ := http.NewRequest("GET", url, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestGetTopicByID_RidNotToday: rid가 오늘 생성된 것이 아니면 포인트 미적립하고 200 반환
func TestGetTopicByID_RidNotToday(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	repo := &mockHistoryRepo{
		topic: &collector.TopicDetail{
			ID:       topicID,
			Category: "top",
			Topic:    "오래된 주제",
		},
		isToday: false, // run은 오늘 생성된 것이 아님
	}
	levelRepo := &mockLevelRepo{points: 0}
	svc := level.NewService(levelRepo)

	r := newTestRouter(repo, svc, &mockSubGetter{})

	w := httptest.NewRecorder()
	url := fmt.Sprintf("/context/topic/%s?rid=%s&uid=%s", topicID, runID, userID)
	req, _ := http.NewRequest("GET", url, nil)
	r.ServeHTTP(w, req)

	// 200 반환, 포인트는 적립 안 됨
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestGetTopicByID_InvalidRID: rid가 UUID 형식이 아니면 포인트 로직을 건너뛰고 200 반환
func TestGetTopicByID_InvalidRID(t *testing.T) {
	topicID := uuid.New()
	userID := uuid.New().String()

	repo := &mockHistoryRepo{
		topic: &collector.TopicDetail{
			ID:       topicID,
			Category: "top",
			Topic:    "테스트",
		},
	}
	svc := level.NewService(&mockLevelRepo{})
	r := newTestRouter(repo, svc, &mockSubGetter{})

	w := httptest.NewRecorder()
	url := fmt.Sprintf("/context/topic/%s?rid=not-valid-uuid&uid=%s", topicID, userID)
	req, _ := http.NewRequest("GET", url, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestGetTopicByID_AlreadyEarned: 이미 동일 run+topic에서 포인트를 적립한 경우 중복 적립 없이 200 반환
func TestGetTopicByID_AlreadyEarned(t *testing.T) {
	topicID := uuid.New()
	runID := uuid.New()
	userID := uuid.New().String()

	repo := &mockHistoryRepo{
		topic: &collector.TopicDetail{
			ID:       topicID,
			Category: "top",
			Topic:    "이미 읽은 주제",
		},
		isToday: true,
	}
	levelRepo := &mockLevelRepo{points: 5, alreadyEarned: true} // 이미 이전에 적립됨
	svc := level.NewService(levelRepo)

	r := newTestRouter(repo, svc, &mockSubGetter{subs: []string{}})

	w := httptest.NewRecorder()
	url := fmt.Sprintf("/context/topic/%s?rid=%s&uid=%s", topicID, runID, userID)
	req, _ := http.NewRequest("GET", url, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// TestGetTopicByID_TopicCategoryInResponse: 응답 body에 category 필드가 포함되어야 함
func TestGetTopicByID_TopicCategoryInResponse(t *testing.T) {
	topicID := uuid.New()
	repo := &mockHistoryRepo{
		topic: &collector.TopicDetail{
			ID:       topicID,
			Category: "entertainment",
			Topic:    "연예 뉴스",
		},
	}
	svc := level.NewService(&mockLevelRepo{})
	r := newTestRouter(repo, svc, &mockSubGetter{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/context/topic/%s", topicID), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'data' key in response")
	}
	if data["category"] != "entertainment" {
		t.Errorf("expected category 'entertainment', got %v", data["category"])
	}
}

// ─── Verify level.CalcPoints point values match expected base values ──────────

// TestCalcPoints_Preferred: 선호 카테고리 기본 포인트 확인
func TestCalcPoints_Preferred(t *testing.T) {
	pts := level.CalcPoints(true, 0)
	if pts != level.BasePointPreferred {
		t.Errorf("expected %d for preferred day=0, got %d", level.BasePointPreferred, pts)
	}
}

// TestCalcPoints_NonPreferred: 비선호 카테고리 기본 포인트 확인
func TestCalcPoints_NonPreferred(t *testing.T) {
	pts := level.CalcPoints(false, 0)
	if pts != level.BasePointNonPreferred {
		t.Errorf("expected %d for non-preferred day=0, got %d", level.BasePointNonPreferred, pts)
	}
}

// TestCalcPoints_BonusAccumulates: 날짜 경과에 따른 보너스 포인트 누적 확인
func TestCalcPoints_BonusAccumulates(t *testing.T) {
	pts := level.CalcPoints(true, 3) // 5 + 3*5 = 20
	expected := level.BasePointPreferred + 3*level.BonusPointPerDay
	if pts != expected {
		t.Errorf("expected %d for preferred day=3, got %d", expected, pts)
	}
}

// TestCalcPoints_MaxCap: 최대 포인트 제한 확인
func TestCalcPoints_MaxCap(t *testing.T) {
	pts := level.CalcPoints(false, 100) // 15 + 100*5 >> 50
	if pts != level.MaxPointEarn {
		t.Errorf("expected max %d, got %d", level.MaxPointEarn, pts)
	}
}

// ─── Verify level.IsPreferredCategory ─────────────────────────────────────────

func TestIsPreferredCategory_Top(t *testing.T) {
	if !level.IsPreferredCategory("top", nil) {
		t.Error("expected 'top' to be preferred")
	}
}

func TestIsPreferredCategory_Brief(t *testing.T) {
	if !level.IsPreferredCategory("brief", nil) {
		t.Error("expected 'brief' to be preferred")
	}
}

func TestIsPreferredCategory_Subscribed(t *testing.T) {
	if !level.IsPreferredCategory("sports", []string{"sports", "economy"}) {
		t.Error("expected 'sports' to be preferred when in subscriptions")
	}
}

func TestIsPreferredCategory_NotSubscribed(t *testing.T) {
	if level.IsPreferredCategory("politics", []string{"sports", "economy"}) {
		t.Error("expected 'politics' to be non-preferred")
	}
}

// Assert that storage.HistoryRepository implements collector.HistoryRepository
// (compile-time check)
var _ collector.HistoryRepository = (*storage.HistoryRepository)(nil)
