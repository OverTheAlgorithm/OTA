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

	"ota/api/handler"
	"ota/auth"
	"ota/domain/terms"
	"ota/domain/user"
)

// ─── Mocks (mockCache is defined in level_handler_test.go) ──────────────────

type mockUserRepoSignup struct {
	users        map[int64]user.User
	upsertedUser user.User
	upsertErr    error
	findByIDUser user.User
	findByIDErr  error
}

func newMockUserRepoSignup() *mockUserRepoSignup {
	return &mockUserRepoSignup{users: make(map[int64]user.User)}
}

func (m *mockUserRepoSignup) UpsertByKakaoID(_ context.Context, kakaoID int64, email, nickname, profileImage string) (user.User, error) {
	if m.upsertErr != nil {
		return user.User{}, m.upsertErr
	}
	u := user.User{
		ID:           "user-" + nickname,
		KakaoID:      kakaoID,
		Email:        email,
		Nickname:     nickname,
		ProfileImage: profileImage,
		Role:         "user",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.upsertedUser = u
	return u, nil
}

func (m *mockUserRepoSignup) FindByID(_ context.Context, _ string) (user.User, error) {
	if m.findByIDErr != nil {
		return user.User{}, m.findByIDErr
	}
	return m.findByIDUser, nil
}

func (m *mockUserRepoSignup) FindByKakaoID(_ context.Context, kakaoID int64) (user.User, bool, error) {
	u, ok := m.users[kakaoID]
	return u, ok, nil
}

func (m *mockUserRepoSignup) FindByEmail(_ context.Context, _ string) (user.User, error) {
	return user.User{}, fmt.Errorf("user not found")
}

func (m *mockUserRepoSignup) UpdateEmail(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *mockUserRepoSignup) DeleteByID(_ context.Context, _ string) error {
	return nil
}

type mockTermsRepoSignup struct {
	requiredTerms []terms.Term
	savedConsents map[string][]string
	saveErr       error
}

func newMockTermsRepoSignup() *mockTermsRepoSignup {
	return &mockTermsRepoSignup{savedConsents: make(map[string][]string)}
}

func (m *mockTermsRepoSignup) Create(_ context.Context, t terms.Term) (terms.Term, error) {
	return t, nil
}
func (m *mockTermsRepoSignup) ListAll(_ context.Context) ([]terms.Term, error) {
	return nil, nil
}
func (m *mockTermsRepoSignup) ListActive(_ context.Context) ([]terms.Term, error) {
	return nil, nil
}
func (m *mockTermsRepoSignup) FindActiveRequired(_ context.Context) ([]terms.Term, error) {
	return m.requiredTerms, nil
}
func (m *mockTermsRepoSignup) SaveConsents(_ context.Context, userID string, termIDs []string) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.savedConsents[userID] = termIDs
	return nil
}
func (m *mockTermsRepoSignup) UpdateActive(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *mockTermsRepoSignup) GetUserConsents(_ context.Context, _ string) ([]terms.UserTermConsent, error) {
	return nil, nil
}

// ─── Test Setup ─────────────────────────────────────────────────────────────

func setupSignupTest(t *testing.T) (*gin.Engine, *mockCache, *mockUserRepoSignup, *mockTermsRepoSignup) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	signupCache := newMockCache()
	userRepo := newMockUserRepoSignup()
	termsRepo := newMockTermsRepoSignup()
	termsSvc := terms.NewService(termsRepo)

	jwtManager := auth.NewJWTManager("test-secret-key-that-is-long-enough")

	h := handler.NewAuthHandler(
		nil, // kakao client
		jwtManager,
		auth.NewStateStore(),
		userRepo,
		nil, // welcome deliverer
		nil, // bonus granter
		0,   // signup bonus
		"http://localhost:5173",
		signupCache,
		termsSvc,
	)

	r := gin.New()
	group := r.Group("/api/v1/auth")
	h.RegisterRoutes(group)

	return r, signupCache, userRepo, termsRepo
}

// ─── Tests ──────────────────────────────────────────────────────────────────

func TestCompleteSignup_Success(t *testing.T) {
	r, cache, userRepo, termsRepo := setupSignupTest(t)

	cache.Set("test-key", handler.PendingSignup{
		KakaoID: 12345, Email: "test@example.com", Nickname: "TestUser",
	}, 3*time.Minute)

	termsRepo.requiredTerms = []terms.Term{
		{ID: "term-1", Title: "Privacy", Version: "1", Required: true},
	}

	body, _ := json.Marshal(map[string]any{
		"signup_key":      "test-key",
		"agreed_term_ids": []string{"term-1"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/complete-signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if userRepo.upsertedUser.KakaoID != 12345 {
		t.Fatalf("expected kakao_id 12345, got %d", userRepo.upsertedUser.KakaoID)
	}

	saved, ok := termsRepo.savedConsents[userRepo.upsertedUser.ID]
	if !ok {
		t.Fatal("expected consents to be saved")
	}
	if len(saved) != 1 || saved[0] != "term-1" {
		t.Fatalf("expected [term-1], got %v", saved)
	}

	if cache.Has("test-key") {
		t.Fatal("expected cache entry to be deleted after signup")
	}

	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "ota_token" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected ota_token cookie to be set")
	}
}

func TestCompleteSignup_ExpiredCache(t *testing.T) {
	r, _, _, _ := setupSignupTest(t)

	body, _ := json.Marshal(map[string]any{
		"signup_key":      "expired-key",
		"agreed_term_ids": []string{"term-1"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/complete-signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCompleteSignup_MissingRequiredTerm(t *testing.T) {
	r, cache, _, termsRepo := setupSignupTest(t)

	cache.Set("test-key", handler.PendingSignup{
		KakaoID: 12345, Email: "test@example.com", Nickname: "TestUser",
	}, 3*time.Minute)

	termsRepo.requiredTerms = []terms.Term{
		{ID: "term-1", Title: "Privacy", Version: "1", Required: true},
		{ID: "term-2", Title: "TOS", Version: "1", Required: true},
	}

	body, _ := json.Marshal(map[string]any{
		"signup_key":      "test-key",
		"agreed_term_ids": []string{"term-1"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/complete-signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	if cache.Has("test-key") {
		t.Fatal("expected cache entry to be deleted even on failure")
	}
}

func TestCompleteSignup_MissingSignupKey(t *testing.T) {
	r, _, _, _ := setupSignupTest(t)

	body, _ := json.Marshal(map[string]any{
		"agreed_term_ids": []string{"term-1"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/complete-signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCompleteSignup_EmptyAgreedWithRequiredTerms(t *testing.T) {
	r, cache, _, termsRepo := setupSignupTest(t)

	cache.Set("test-key", handler.PendingSignup{
		KakaoID: 12345, Email: "test@example.com", Nickname: "TestUser",
	}, 3*time.Minute)

	termsRepo.requiredTerms = []terms.Term{
		{ID: "term-1", Title: "Privacy", Version: "1", Required: true},
	}

	body, _ := json.Marshal(map[string]any{
		"signup_key":      "test-key",
		"agreed_term_ids": []string{},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/complete-signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCompleteSignup_NoRequiredTerms_SucceedsWithEmptyAgreement(t *testing.T) {
	r, cache, _, termsRepo := setupSignupTest(t)

	cache.Set("test-key", handler.PendingSignup{
		KakaoID: 12345, Email: "test@example.com", Nickname: "TestUser",
	}, 3*time.Minute)

	termsRepo.requiredTerms = []terms.Term{}

	body, _ := json.Marshal(map[string]any{
		"signup_key":      "test-key",
		"agreed_term_ids": []string{},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/complete-signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCompleteSignup_CacheBustedOnFailure(t *testing.T) {
	r, cache, _, termsRepo := setupSignupTest(t)

	cache.Set("test-key", handler.PendingSignup{
		KakaoID: 12345, Email: "test@example.com", Nickname: "TestUser",
	}, 3*time.Minute)

	termsRepo.requiredTerms = []terms.Term{
		{ID: "term-1", Title: "Privacy", Version: "1", Required: true},
	}

	body, _ := json.Marshal(map[string]any{
		"signup_key":      "test-key",
		"agreed_term_ids": []string{},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/complete-signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if cache.Has("test-key") {
		t.Fatal("expected cache to be busted after failed validation")
	}

	// Second attempt with same key should get expired error
	body2, _ := json.Marshal(map[string]any{
		"signup_key":      "test-key",
		"agreed_term_ids": []string{"term-1"},
	})
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/v1/auth/complete-signup", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on second attempt, got %d", w2.Code)
	}
}
