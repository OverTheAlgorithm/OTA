package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ota/api/handler"
	"ota/domain/collector"
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
	recentTopics []collector.TopicPreview
	recentErr    error
}

func (m *mockHistoryRepo) GetHistoryForUser(_ context.Context, _ string, _, _ int) ([]collector.HistoryEntry, bool, error) {
	return m.historyItems, false, m.historyErr
}

func (m *mockHistoryRepo) GetContextItemByID(_ context.Context, _ uuid.UUID) (*collector.TopicDetail, error) {
	return m.topic, m.topicErr
}

func (m *mockHistoryRepo) IsRunCreatedToday(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.isToday, m.isTodayErr
}

func (m *mockHistoryRepo) GetRecentTopics(_ context.Context, _ int) ([]collector.TopicPreview, error) {
	return m.recentTopics, m.recentErr
}

func (m *mockHistoryRepo) GetAllTopics(_ context.Context, _, _ string, _, _ int) ([]collector.TopicPreview, bool, error) {
	return nil, false, nil
}

func (m *mockHistoryRepo) GetItemCategoryMap(_ context.Context, _ []uuid.UUID) (map[uuid.UUID]collector.ItemMeta, error) {
	return nil, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func newTestRouter(repo collector.HistoryRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewContextHistoryHandler(repo, func(c *gin.Context) { c.Next() })
	h.RegisterRoutes(r.Group("/context"))
	return r
}

// ─── Tests for GetTopicByID ───────────────────────────────────────────────────

// TestGetTopicByID_NotFound: 존재하지 않는 topic 요청 시 404 반환
func TestGetTopicByID_NotFound(t *testing.T) {
	repo := &mockHistoryRepo{topic: nil}
	r := newTestRouter(repo)

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
	r := newTestRouter(repo)

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
	r := newTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/context/topic/%s", uuid.New()), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestGetTopicByID_Success: 정상 조회 시 200 반환
func TestGetTopicByID_Success(t *testing.T) {
	topicID := uuid.New()
	repo := &mockHistoryRepo{
		topic: &collector.TopicDetail{
			ID:       topicID,
			Category: "top",
			Topic:    "테스트 주제",
		},
	}
	r := newTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/context/topic/%s", topicID), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
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
	r := newTestRouter(repo)

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

// TestGetTopicByID_NoEarnResult: 응답에 earn_result 필드가 없어야 함
func TestGetTopicByID_NoEarnResult(t *testing.T) {
	topicID := uuid.New()
	repo := &mockHistoryRepo{
		topic: &collector.TopicDetail{
			ID:       topicID,
			Category: "top",
			Topic:    "테스트",
		},
	}
	r := newTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/context/topic/%s?rid=%s&uid=%s", topicID, uuid.New(), uuid.New()), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, exists := resp["earn_result"]; exists {
		t.Error("expected no 'earn_result' key in response after refactor")
	}
}

// ─── Tests for GetRecentTopics ──────────────────────────────────────────────

// TestGetRecentTopics_Success: 최신 뉴스 3개 정상 반환
func TestGetRecentTopics_Success(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	imgURL := "/api/v1/images/test.png"
	repo := &mockHistoryRepo{
		recentTopics: []collector.TopicPreview{
			{ID: id1, Topic: "뉴스 1", Summary: "요약 1", ImageURL: &imgURL},
			{ID: id2, Topic: "뉴스 2", Summary: "요약 2", ImageURL: nil},
		},
	}
	r := newTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/context/recent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data []collector.TopicPreview `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 topics, got %d", len(resp.Data))
	}
	if resp.Data[0].Topic != "뉴스 1" {
		t.Errorf("expected topic '뉴스 1', got %q", resp.Data[0].Topic)
	}
}

// TestGetRecentTopics_Empty: collection run이 없을 때 빈 배열 반환
func TestGetRecentTopics_Empty(t *testing.T) {
	repo := &mockHistoryRepo{recentTopics: nil}
	r := newTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/context/recent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data []collector.TopicPreview `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected 0 topics, got %d", len(resp.Data))
	}
}

// TestGetRecentTopics_RepoError: DB 오류 시에도 빈 배열로 200 반환 (랜딩 페이지가 깨지지 않도록)
func TestGetRecentTopics_RepoError(t *testing.T) {
	repo := &mockHistoryRepo{recentErr: errors.New("db error")}
	r := newTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/context/recent", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even on error, got %d", w.Code)
	}

	var resp struct {
		Data []collector.TopicPreview `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected 0 topics on error, got %d", len(resp.Data))
	}
}

// ─── Verify level.CalcCoins coin values match expected base values ──────────

// (CalcCoins/IsPreferredCategory tests remain in level package tests)

// Assert that storage.HistoryRepository implements collector.HistoryRepository
// (compile-time check)
var _ collector.HistoryRepository = (*storage.HistoryRepository)(nil)
