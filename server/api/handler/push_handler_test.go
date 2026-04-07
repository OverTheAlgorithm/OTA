package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ota/api/handler"
	"ota/domain/push"
)

// ─── Mock Repository ─────────────────────────────────────────────────────────

type mockPushRepoForTokenHandler struct {
	saved       []push.PushToken
	saveErr     error
	unlinkCalls []unlinkCall
	unlinkErr   error
	deletedTokens []string
}

type unlinkCall struct {
	userID string
	token  string
}

func (m *mockPushRepoForTokenHandler) Save(_ context.Context, t push.PushToken) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.saved = append(m.saved, t)
	return nil
}

func (m *mockPushRepoForTokenHandler) UnlinkUser(_ context.Context, userID, token string) error {
	if m.unlinkErr != nil {
		return m.unlinkErr
	}
	m.unlinkCalls = append(m.unlinkCalls, unlinkCall{userID, token})
	return nil
}

func (m *mockPushRepoForTokenHandler) DeleteByTokens(_ context.Context, tokens []string) error {
	m.deletedTokens = append(m.deletedTokens, tokens...)
	return nil
}

func (m *mockPushRepoForTokenHandler) GetByUserID(_ context.Context, _ string) ([]push.PushToken, error) {
	return nil, nil
}

func (m *mockPushRepoForTokenHandler) GetAllActive(_ context.Context) ([]push.PushToken, error) {
	return nil, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// noopMW is a no-op middleware (simulates no auth).
func noopMW() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

func newPushTokenTestRouter(repo push.Repository, authUserID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := push.NewService(repo)

	// OptionalAuth: if authUserID is set, inject it; otherwise no-op.
	optionalAuth := noopMW()
	if authUserID != "" {
		optionalAuth = fakeAuthMW(authUserID)
	}
	// Auth: always inject authUserID (DELETE requires auth).
	authMW := fakeAuthMW(authUserID)

	r := gin.New()
	h := handler.NewPushHandler(svc, optionalAuth, authMW)
	h.RegisterRoutes(r.Group("/mobile/push-token"))
	return r
}

func pushRequest(r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
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

// ─── POST Tests: Anonymous Registration ──────────────────────────────────────

func TestPushHandler_POST_Anonymous_Success(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "") // no auth

	w := pushRequest(r, http.MethodPost, "/mobile/push-token", map[string]string{
		"token":    "ExponentPushToken[abc123]",
		"platform": "android",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 saved token, got %d", len(repo.saved))
	}
	if repo.saved[0].UserID != nil {
		t.Errorf("expected nil user_id for anonymous, got %v", *repo.saved[0].UserID)
	}
	if repo.saved[0].Token != "ExponentPushToken[abc123]" {
		t.Errorf("expected token ExponentPushToken[abc123], got %s", repo.saved[0].Token)
	}
	if repo.saved[0].Platform != "android" {
		t.Errorf("expected platform android, got %s", repo.saved[0].Platform)
	}
}

func TestPushHandler_POST_Anonymous_DefaultPlatform(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "")

	w := pushRequest(r, http.MethodPost, "/mobile/push-token", map[string]string{
		"token": "ExponentPushToken[abc123]",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.saved[0].Platform != "expo" {
		t.Errorf("expected default platform 'expo', got %s", repo.saved[0].Platform)
	}
}

// ─── POST Tests: Authenticated Registration ──────────────────────────────────

func TestPushHandler_POST_Authenticated_LinksUser(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "user-123")

	w := pushRequest(r, http.MethodPost, "/mobile/push-token", map[string]string{
		"token":    "ExponentPushToken[def456]",
		"platform": "ios",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(repo.saved) != 1 {
		t.Fatalf("expected 1 saved token, got %d", len(repo.saved))
	}
	if repo.saved[0].UserID == nil {
		t.Fatal("expected non-nil user_id for authenticated request")
	}
	if *repo.saved[0].UserID != "user-123" {
		t.Errorf("expected user_id user-123, got %s", *repo.saved[0].UserID)
	}
}

func TestPushHandler_POST_Authenticated_OverwritesUser(t *testing.T) {
	// Simulates: device was linked to user-A, now user-B logs in on same device.
	// The ON CONFLICT (token) DO UPDATE ensures user_id is overwritten.
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "user-B")

	w := pushRequest(r, http.MethodPost, "/mobile/push-token", map[string]string{
		"token":    "ExponentPushToken[shared-device]",
		"platform": "android",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if *repo.saved[0].UserID != "user-B" {
		t.Errorf("expected user_id user-B, got %s", *repo.saved[0].UserID)
	}
}

// ─── POST Tests: Validation ──────────────────────────────────────────────────

func TestPushHandler_POST_MissingToken_Returns400(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "")

	w := pushRequest(r, http.MethodPost, "/mobile/push-token", map[string]string{
		"platform": "android",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushHandler_POST_EmptyToken_Returns400(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "")

	w := pushRequest(r, http.MethodPost, "/mobile/push-token", map[string]string{
		"token": "   ",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushHandler_POST_NoBody_Returns400(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "")

	w := pushRequest(r, http.MethodPost, "/mobile/push-token", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── DELETE Tests: Unlink ────────────────────────────────────────────────────

func TestPushHandler_DELETE_Unlinks_UserID(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "user-123")

	w := pushRequest(r, http.MethodDelete, "/mobile/push-token", map[string]string{
		"token": "ExponentPushToken[abc123]",
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(repo.unlinkCalls) != 1 {
		t.Fatalf("expected 1 unlink call, got %d", len(repo.unlinkCalls))
	}
	if repo.unlinkCalls[0].userID != "user-123" {
		t.Errorf("expected unlink for user-123, got %s", repo.unlinkCalls[0].userID)
	}
	if repo.unlinkCalls[0].token != "ExponentPushToken[abc123]" {
		t.Errorf("expected unlink for token ExponentPushToken[abc123], got %s", repo.unlinkCalls[0].token)
	}
}

func TestPushHandler_DELETE_MissingToken_Returns400(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "user-123")

	w := pushRequest(r, http.MethodDelete, "/mobile/push-token", map[string]string{})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushHandler_DELETE_EmptyToken_Returns400(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "user-123")

	w := pushRequest(r, http.MethodDelete, "/mobile/push-token", map[string]string{
		"token": "  ",
	})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── Idempotency: Multiple registrations for same token ──────────────────────

func TestPushHandler_POST_Idempotent_SameToken(t *testing.T) {
	// Calling register twice with the same token should succeed both times.
	// The server upserts on token.
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "user-123")

	body := map[string]string{
		"token":    "ExponentPushToken[same]",
		"platform": "ios",
	}

	w1 := pushRequest(r, http.MethodPost, "/mobile/push-token", body)
	w2 := pushRequest(r, http.MethodPost, "/mobile/push-token", body)

	if w1.Code != http.StatusOK || w2.Code != http.StatusOK {
		t.Fatalf("expected both 200, got %d and %d", w1.Code, w2.Code)
	}
	if len(repo.saved) != 2 {
		t.Errorf("expected 2 save calls (upsert), got %d", len(repo.saved))
	}
}

// ─── Multi-device scenario ───────────────────────────────────────────────────

func TestPushHandler_POST_MultiDevice_SameUser(t *testing.T) {
	// Same user registers from two different devices.
	repo := &mockPushRepoForTokenHandler{}
	r := newPushTokenTestRouter(repo, "user-123")

	w1 := pushRequest(r, http.MethodPost, "/mobile/push-token", map[string]string{
		"token":    "ExponentPushToken[device-A]",
		"platform": "android",
	})
	w2 := pushRequest(r, http.MethodPost, "/mobile/push-token", map[string]string{
		"token":    "ExponentPushToken[device-B]",
		"platform": "ios",
	})

	if w1.Code != http.StatusOK || w2.Code != http.StatusOK {
		t.Fatalf("expected both 200, got %d and %d", w1.Code, w2.Code)
	}
	if len(repo.saved) != 2 {
		t.Fatalf("expected 2 saved tokens, got %d", len(repo.saved))
	}
	if repo.saved[0].Token == repo.saved[1].Token {
		t.Error("expected different tokens for different devices")
	}
	if *repo.saved[0].UserID != "user-123" || *repo.saved[1].UserID != "user-123" {
		t.Error("expected both tokens linked to user-123")
	}
}

// ─── Full lifecycle: anonymous → login → logout ──────────────────────────────

func TestPushHandler_FullLifecycle(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{}
	token := "ExponentPushToken[lifecycle]"

	// Step 1: Anonymous registration (app start, no auth).
	rAnon := newPushTokenTestRouter(repo, "")
	w := pushRequest(rAnon, http.MethodPost, "/mobile/push-token", map[string]string{
		"token":    token,
		"platform": "android",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("Step 1 (anon register): expected 200, got %d", w.Code)
	}
	if repo.saved[0].UserID != nil {
		t.Fatalf("Step 1: expected nil user_id, got %v", *repo.saved[0].UserID)
	}

	// Step 2: User logs in → re-register with auth (links user).
	rAuth := newPushTokenTestRouter(repo, "user-42")
	w = pushRequest(rAuth, http.MethodPost, "/mobile/push-token", map[string]string{
		"token":    token,
		"platform": "android",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("Step 2 (auth register): expected 200, got %d", w.Code)
	}
	if *repo.saved[1].UserID != "user-42" {
		t.Fatalf("Step 2: expected user_id user-42, got %s", *repo.saved[1].UserID)
	}

	// Step 3: User logs out → unlink (user_id set to NULL, token preserved).
	w = pushRequest(rAuth, http.MethodDelete, "/mobile/push-token", map[string]string{
		"token": token,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("Step 3 (unlink): expected 200, got %d", w.Code)
	}
	if len(repo.unlinkCalls) != 1 {
		t.Fatalf("Step 3: expected 1 unlink call, got %d", len(repo.unlinkCalls))
	}
	if repo.unlinkCalls[0].userID != "user-42" {
		t.Errorf("Step 3: expected unlink user_id user-42, got %s", repo.unlinkCalls[0].userID)
	}
}

// ─── Server error handling ───────────────────────────────────────────────────

func TestPushHandler_POST_ServerError_Returns500(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{saveErr: context.DeadlineExceeded}
	r := newPushTokenTestRouter(repo, "")

	w := pushRequest(r, http.MethodPost, "/mobile/push-token", map[string]string{
		"token": "ExponentPushToken[err]",
	})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPushHandler_DELETE_ServerError_Returns500(t *testing.T) {
	repo := &mockPushRepoForTokenHandler{unlinkErr: context.DeadlineExceeded}
	r := newPushTokenTestRouter(repo, "user-123")

	w := pushRequest(r, http.MethodDelete, "/mobile/push-token", map[string]string{
		"token": "ExponentPushToken[err]",
	})

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}
