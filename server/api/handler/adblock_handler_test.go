package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ota/api/handler"
)

// ─── Mock Repository ──────────────────────────────────────────────────────────

type mockAdblockRepoForHandler struct {
	calledUserID   string
	calledDetected bool
	err            error
}

func (m *mockAdblockRepoForHandler) UpdateAdblockStatus(_ context.Context, userID string, detected bool) error {
	m.calledUserID = userID
	m.calledDetected = detected
	return m.err
}

// ─── Helper ───────────────────────────────────────────────────────────────────

func newAdblockTestRouter(repo handler.AdblockRepository, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewAdblockHandler(repo)
	group := r.Group("/adblock", fakeAuthMW(userID))
	h.RegisterRoutes(group)
	return r
}

func adblockRequest(r *gin.Engine, body any) *httptest.ResponseRecorder {
	var buf *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	} else {
		buf = bytes.NewReader(nil)
	}
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/adblock/report", buf)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// ─── Tests ────────────────────────────────────────────────────────────────────

func TestAdblockHandler_ReportDetected(t *testing.T) {
	repo := &mockAdblockRepoForHandler{}
	r := newAdblockTestRouter(repo, "test-user-id")

	w := adblockRequest(r, map[string]any{"detected": true})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.calledUserID != "test-user-id" {
		t.Errorf("expected calledUserID=test-user-id, got %q", repo.calledUserID)
	}
	if !repo.calledDetected {
		t.Error("expected calledDetected=true")
	}
}

func TestAdblockHandler_ReportNotDetected(t *testing.T) {
	repo := &mockAdblockRepoForHandler{}
	r := newAdblockTestRouter(repo, "test-user-id")

	w := adblockRequest(r, map[string]any{"detected": false})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.calledUserID != "test-user-id" {
		t.Errorf("expected calledUserID=test-user-id, got %q", repo.calledUserID)
	}
	if repo.calledDetected {
		t.Error("expected calledDetected=false")
	}
}

func TestAdblockHandler_InvalidBody(t *testing.T) {
	repo := &mockAdblockRepoForHandler{}
	r := newAdblockTestRouter(repo, "test-user-id")

	// Send raw invalid JSON (not a valid object for ShouldBindJSON).
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/adblock/report", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdblockHandler_RepoError(t *testing.T) {
	repo := &mockAdblockRepoForHandler{err: errors.New("db error")}
	r := newAdblockTestRouter(repo, "test-user-id")

	w := adblockRequest(r, map[string]any{"detected": true})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}
