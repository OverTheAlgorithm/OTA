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
	"ota/domain/push"
	"ota/scheduler"
)

// ─── Mock ScheduledRepo for handler tests ─────────────────────────────────────

type mockScheduledRepoForHandler struct {
	created      push.ScheduledPush
	createErr    error
	byID         push.ScheduledPush
	getErr       error
	updateErr    error
	listed       []push.ScheduledPush
	listErr      error
	markSentOk   bool
	markSentErr  error
	markCancelOk bool
	markCancelErr error
	markFailedOk bool
}

func (m *mockScheduledRepoForHandler) Create(_ context.Context, p push.ScheduledPush) (push.ScheduledPush, error) {
	if m.createErr != nil {
		return push.ScheduledPush{}, m.createErr
	}
	m.created = p
	return p, nil
}

func (m *mockScheduledRepoForHandler) GetByID(_ context.Context, _ uuid.UUID) (push.ScheduledPush, error) {
	return m.byID, m.getErr
}

func (m *mockScheduledRepoForHandler) Update(_ context.Context, _ push.ScheduledPush) error {
	return m.updateErr
}

func (m *mockScheduledRepoForHandler) List(_ context.Context, status *string) ([]push.ScheduledPush, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	if status == nil {
		return m.listed, nil
	}
	var filtered []push.ScheduledPush
	for _, p := range m.listed {
		if string(p.Status) == *status {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}

func (m *mockScheduledRepoForHandler) ListPending(_ context.Context) ([]push.ScheduledPush, error) {
	return m.listed, m.listErr
}

func (m *mockScheduledRepoForHandler) MarkSent(_ context.Context, _ uuid.UUID, _ time.Time) (bool, error) {
	return m.markSentOk, m.markSentErr
}

func (m *mockScheduledRepoForHandler) MarkFailed(_ context.Context, _ uuid.UUID, _ string) (bool, error) {
	return m.markFailedOk, nil
}

func (m *mockScheduledRepoForHandler) MarkCancelled(_ context.Context, _ uuid.UUID) (bool, error) {
	return m.markCancelOk, m.markCancelErr
}

// ─── Mock PushExecutor for scheduler (no-op) ──────────────────────────────────

type mockPushExecutorForHandler struct{}

func (m *mockPushExecutorForHandler) ExecuteBySchedule(_ context.Context, _ uuid.UUID) error {
	return nil
}

// ─── Mock PushRepo for push.Service (returns empty tokens → no HTTP calls) ───

type mockPushRepoForHandler struct{}

func (m *mockPushRepoForHandler) Save(_ context.Context, _ push.PushToken) error          { return nil }
func (m *mockPushRepoForHandler) UnlinkUser(_ context.Context, _, _ string) error         { return nil }
func (m *mockPushRepoForHandler) DeleteByTokens(_ context.Context, _ []string) error { return nil }
func (m *mockPushRepoForHandler) GetByUserID(_ context.Context, _ string) ([]push.PushToken, error) {
	return nil, nil
}
func (m *mockPushRepoForHandler) GetAllActive(_ context.Context) ([]push.PushToken, error) {
	return []push.PushToken{}, nil
}

// ─── Router builder ───────────────────────────────────────────────────────────

func newPushAdminTestRouter(scheduledRepo push.ScheduledRepository, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	pushSvc := push.NewService(&mockPushRepoForHandler{})
	scheduledSvc := push.NewScheduledService(scheduledRepo, pushSvc)

	executor := &mockPushExecutorForHandler{}
	ps := scheduler.NewPushScheduler(executor, context.Background())

	r := gin.New()
	h := handler.NewPushAdminHandler(scheduledSvc, ps)
	group := r.Group("/admin/push", fakeAuthMW(userID))
	h.RegisterRoutes(group)
	return r
}

// ─── HTTP helpers ─────────────────────────────────────────────────────────────

func doRequest(r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var buf *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, path, buf)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// ─── Create Tests ─────────────────────────────────────────────────────────────

func TestPushAdminHandler_Create_Valid(t *testing.T) {
	repo := &mockScheduledRepoForHandler{}
	r := newPushAdminTestRouter(repo, "admin-user")

	futureTime := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	body := map[string]any{
		"title":        "Hello",
		"body":         "World",
		"scheduled_at": futureTime,
	}

	w := doRequest(r, http.MethodPost, "/admin/push", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushAdminHandler_Create_MissingTitle(t *testing.T) {
	repo := &mockScheduledRepoForHandler{}
	r := newPushAdminTestRouter(repo, "admin-user")

	body := map[string]any{
		"body": "World",
	}

	w := doRequest(r, http.MethodPost, "/admin/push", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing title, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushAdminHandler_Create_MissingBody(t *testing.T) {
	repo := &mockScheduledRepoForHandler{}
	r := newPushAdminTestRouter(repo, "admin-user")

	body := map[string]any{
		"title": "Hello",
	}

	w := doRequest(r, http.MethodPost, "/admin/push", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing body, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushAdminHandler_Create_DataTooLarge(t *testing.T) {
	repo := &mockScheduledRepoForHandler{}
	r := newPushAdminTestRouter(repo, "admin-user")

	// Build a data field exceeding 4KB
	bigValue := fmt.Sprintf("%s", make([]byte, 5000))
	body := map[string]any{
		"title": "Hello",
		"body":  "World",
		"data":  map[string]any{"payload": bigValue},
	}

	w := doRequest(r, http.MethodPost, "/admin/push", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for large data, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── Update Tests ─────────────────────────────────────────────────────────────

func TestPushAdminHandler_Update_Valid(t *testing.T) {
	id := uuid.New()
	repo := &mockScheduledRepoForHandler{
		byID: push.ScheduledPush{ID: id, Status: push.StatusPending},
	}
	r := newPushAdminTestRouter(repo, "admin-user")

	body := map[string]any{
		"title": "Updated Title",
		"body":  "Updated Body",
	}

	w := doRequest(r, http.MethodPut, "/admin/push/"+id.String(), body)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushAdminHandler_Update_NonPending_Conflict(t *testing.T) {
	id := uuid.New()
	repo := &mockScheduledRepoForHandler{
		byID: push.ScheduledPush{ID: id, Status: push.StatusSent},
	}
	r := newPushAdminTestRouter(repo, "admin-user")

	body := map[string]any{
		"title": "New",
		"body":  "Body",
	}

	w := doRequest(r, http.MethodPut, "/admin/push/"+id.String(), body)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for non-pending, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushAdminHandler_Update_InvalidID(t *testing.T) {
	repo := &mockScheduledRepoForHandler{}
	r := newPushAdminTestRouter(repo, "admin-user")

	body := map[string]any{"title": "T", "body": "B"}
	w := doRequest(r, http.MethodPut, "/admin/push/not-a-uuid", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", w.Code)
	}
}

// ─── Delete Tests ─────────────────────────────────────────────────────────────

func TestPushAdminHandler_Delete_Valid(t *testing.T) {
	id := uuid.New()
	repo := &mockScheduledRepoForHandler{markCancelOk: true}
	r := newPushAdminTestRouter(repo, "admin-user")

	w := doRequest(r, http.MethodDelete, "/admin/push/"+id.String(), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushAdminHandler_Delete_NonPending(t *testing.T) {
	id := uuid.New()
	repo := &mockScheduledRepoForHandler{markCancelOk: false}
	r := newPushAdminTestRouter(repo, "admin-user")

	w := doRequest(r, http.MethodDelete, "/admin/push/"+id.String(), nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for non-pending, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushAdminHandler_Delete_InvalidID(t *testing.T) {
	repo := &mockScheduledRepoForHandler{}
	r := newPushAdminTestRouter(repo, "admin-user")

	w := doRequest(r, http.MethodDelete, "/admin/push/not-a-uuid", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", w.Code)
	}
}

// ─── ExecuteNow Tests ─────────────────────────────────────────────────────────

func TestPushAdminHandler_ExecuteNow_Success(t *testing.T) {
	id := uuid.New()
	repo := &mockScheduledRepoForHandler{
		byID:       push.ScheduledPush{ID: id, Title: "t", Body: "b", Status: push.StatusPending},
		markSentOk: true,
	}
	r := newPushAdminTestRouter(repo, "admin-user")

	w := doRequest(r, http.MethodPost, "/admin/push/"+id.String()+"/send", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushAdminHandler_ExecuteNow_InvalidID(t *testing.T) {
	repo := &mockScheduledRepoForHandler{}
	r := newPushAdminTestRouter(repo, "admin-user")

	w := doRequest(r, http.MethodPost, "/admin/push/not-a-uuid/send", nil)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", w.Code)
	}
}

// ─── List Tests ───────────────────────────────────────────────────────────────

func TestPushAdminHandler_List_ReturnsArray(t *testing.T) {
	repo := &mockScheduledRepoForHandler{
		listed: []push.ScheduledPush{
			{ID: uuid.New(), Status: push.StatusPending},
			{ID: uuid.New(), Status: push.StatusSent},
		},
	}
	r := newPushAdminTestRouter(repo, "admin-user")

	w := doRequest(r, http.MethodGet, "/admin/push", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []push.ScheduledPush `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Data))
	}
}

func TestPushAdminHandler_List_StatusFilter(t *testing.T) {
	repo := &mockScheduledRepoForHandler{
		listed: []push.ScheduledPush{
			{ID: uuid.New(), Status: push.StatusPending},
			{ID: uuid.New(), Status: push.StatusSent},
		},
	}
	r := newPushAdminTestRouter(repo, "admin-user")

	w := doRequest(r, http.MethodGet, "/admin/push?status=pending", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data []push.ScheduledPush `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Errorf("expected 1 pending item, got %d", len(resp.Data))
	}
}
