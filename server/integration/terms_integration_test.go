package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"ota/api"
	"ota/api/handler"
	"ota/auth"
	"ota/cache"
	"ota/domain/terms"
	"ota/platform/kakao"
	"ota/storage"
)

func TestTerms_CRUD_Integration(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "user_term_consents", "terms")

	termsRepo := storage.NewTermsRepository(db.Pool)
	termsSvc := terms.NewService(termsRepo)
	adminHandler := handler.NewTermsAdminHandler(termsSvc)
	publicHandler := handler.NewTermsHandler(termsSvc)

	gin.SetMode(gin.TestMode)
	jwtManager := auth.NewJWTManager("test-secret")
	router := api.NewRouter("api", "v1", "http://localhost:5173", jwtManager, []api.RouteModule{
		{GroupName: "admin/terms", Handler: adminHandler, Middlewares: []gin.HandlerFunc{}},
		{GroupName: "terms", Handler: publicHandler, Middlewares: []gin.HandlerFunc{}},
	})

	// 1. Create a required active term
	body, _ := json.Marshal(map[string]any{
		"title":       "개인정보 처리방침",
		"description": "개인정보 수집 및 이용에 대한 안내",
		"url":         "https://notion.so/privacy",
		"version":     "1",
		"active":      true,
		"required":    true,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/terms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create term: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var createResp struct {
		Data terms.Term `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &createResp)
	termID := createResp.Data.ID
	if termID == "" {
		t.Fatal("expected non-empty term ID")
	}

	// 2. Create an optional active term
	body2, _ := json.Marshal(map[string]any{
		"title":    "마케팅 수신 동의",
		"url":      "https://notion.so/marketing",
		"version":  "1",
		"active":   true,
		"required": false,
	})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/admin/terms", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("create optional term: expected 201, got %d: %s", w2.Code, w2.Body.String())
	}

	// 3. Create an inactive term
	body3, _ := json.Marshal(map[string]any{
		"title":    "이전 이용약관",
		"url":      "https://notion.so/old-tos",
		"version":  "0.9",
		"active":   false,
		"required": true,
	})
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", "/api/v1/admin/terms", bytes.NewReader(body3))
	req3.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusCreated {
		t.Fatalf("create inactive term: expected 201, got %d", w3.Code)
	}

	// 4. List all terms (admin) — should be 3
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("GET", "/api/v1/admin/terms", nil)
	router.ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("list all: expected 200, got %d", w4.Code)
	}

	var listAllResp struct {
		Data []terms.Term `json:"data"`
	}
	json.Unmarshal(w4.Body.Bytes(), &listAllResp)
	if len(listAllResp.Data) != 3 {
		t.Fatalf("list all: expected 3, got %d", len(listAllResp.Data))
	}

	// 5. List active terms (public) — should be 2
	w5 := httptest.NewRecorder()
	req5, _ := http.NewRequest("GET", "/api/v1/terms/active", nil)
	router.ServeHTTP(w5, req5)

	if w5.Code != http.StatusOK {
		t.Fatalf("list active: expected 200, got %d", w5.Code)
	}

	var listActiveResp struct {
		Data []terms.Term `json:"data"`
	}
	json.Unmarshal(w5.Body.Bytes(), &listActiveResp)
	if len(listActiveResp.Data) != 2 {
		t.Fatalf("list active: expected 2, got %d", len(listActiveResp.Data))
	}

	// 6. Toggle active status — deactivate the first term
	toggleBody, _ := json.Marshal(map[string]any{"active": false})
	w6 := httptest.NewRecorder()
	req6, _ := http.NewRequest("PATCH", "/api/v1/admin/terms/"+termID+"/active", bytes.NewReader(toggleBody))
	req6.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w6, req6)

	if w6.Code != http.StatusOK {
		t.Fatalf("toggle active: expected 200, got %d: %s", w6.Code, w6.Body.String())
	}

	// 7. List active again — should now be 1
	w7 := httptest.NewRecorder()
	req7, _ := http.NewRequest("GET", "/api/v1/terms/active", nil)
	router.ServeHTTP(w7, req7)

	var listActive2 struct {
		Data []terms.Term `json:"data"`
	}
	json.Unmarshal(w7.Body.Bytes(), &listActive2)
	if len(listActive2.Data) != 1 {
		t.Fatalf("after toggle: expected 1 active, got %d", len(listActive2.Data))
	}

	// 8. Duplicate title+version should fail
	dupBody, _ := json.Marshal(map[string]any{
		"title":    "개인정보 처리방침",
		"url":      "https://other.com",
		"version":  "1",
		"active":   true,
		"required": true,
	})
	w8 := httptest.NewRecorder()
	req8, _ := http.NewRequest("POST", "/api/v1/admin/terms", bytes.NewReader(dupBody))
	req8.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w8, req8)

	if w8.Code != http.StatusBadRequest {
		t.Fatalf("duplicate: expected 400, got %d: %s", w8.Code, w8.Body.String())
	}
}

func TestTerms_ConsentValidation_Integration(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "user_term_consents", "terms", "users")

	termsRepo := storage.NewTermsRepository(db.Pool)
	termsSvc := terms.NewService(termsRepo)
	ctx := context.Background()

	// Create two required terms + one optional
	t1, err := termsRepo.Create(ctx, terms.Term{Title: "Privacy", URL: "https://a.com", Version: "1", Active: true, Required: true})
	if err != nil {
		t.Fatal(err)
	}
	t2, err := termsRepo.Create(ctx, terms.Term{Title: "TOS", URL: "https://b.com", Version: "1", Active: true, Required: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = termsRepo.Create(ctx, terms.Term{Title: "Marketing", URL: "https://c.com", Version: "1", Active: true, Required: false})
	if err != nil {
		t.Fatal(err)
	}

	// Missing one required — should fail
	err = termsSvc.ValidateConsents(ctx, []string{t1.ID})
	if err == nil {
		t.Fatal("expected validation error for missing required term")
	}

	// All required agreed — should pass
	err = termsSvc.ValidateConsents(ctx, []string{t1.ID, t2.ID})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}

	// Save consents for a test user
	userRepo := storage.NewUserRepository(db.Pool)
	u, err := userRepo.UpsertByKakaoID(ctx, 99999, "consent@test.com", "ConsentUser", "")
	if err != nil {
		t.Fatal(err)
	}

	err = termsRepo.SaveConsents(ctx, u.ID, []string{t1.ID, t2.ID})
	if err != nil {
		t.Fatalf("save consents: %v", err)
	}

	// Verify consents
	consents, err := termsRepo.GetUserConsents(ctx, u.ID)
	if err != nil {
		t.Fatalf("get consents: %v", err)
	}
	if len(consents) != 2 {
		t.Fatalf("expected 2 consents, got %d", len(consents))
	}

	// Idempotency — saving same consents again should not error
	err = termsRepo.SaveConsents(ctx, u.ID, []string{t1.ID, t2.ID})
	if err != nil {
		t.Fatalf("idempotent save: %v", err)
	}

	consents2, _ := termsRepo.GetUserConsents(ctx, u.ID)
	if len(consents2) != 2 {
		t.Fatalf("after idempotent save: expected 2, got %d", len(consents2))
	}
}

func TestTerms_CompleteSignupFlow_Integration(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "user_term_consents", "terms", "users")

	termsRepo := storage.NewTermsRepository(db.Pool)
	termsSvc := terms.NewService(termsRepo)
	userRepo := storage.NewUserRepository(db.Pool)
	ctx := context.Background()

	// Create required term
	term, err := termsRepo.Create(ctx, terms.Term{
		Title: "Privacy", URL: "https://a.com", Version: "1", Active: true, Required: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Setup signup cache + auth handler
	signupCache, err := cache.New(100)
	if err != nil {
		t.Fatal(err)
	}

	jwtManager := auth.NewJWTManager("test-secret-key-long-enough-here")
	kakaoClient := kakao.NewClient("dummy", "dummy", "http://localhost/cb")
	authHandler := handler.NewAuthHandler(
		kakaoClient, jwtManager, auth.NewStateStore(),
		userRepo, nil, nil, 0, "http://localhost:5173",
		signupCache, termsSvc,
	)

	gin.SetMode(gin.TestMode)
	router := api.NewRouter("api", "v1", "http://localhost:5173", jwtManager, []api.RouteModule{
		{GroupName: "auth", Handler: authHandler, Middlewares: []gin.HandlerFunc{}},
	})

	// Simulate cache entry (as if KakaoCallback placed it)
	signupCache.Set("test-signup-key", handler.PendingSignup{
		KakaoID: 77777, Email: "new@user.com", Nickname: "NewUser",
	}, 3*time.Minute)

	// Complete signup with correct consents
	body, _ := json.Marshal(map[string]any{
		"signup_key":      "test-signup-key",
		"agreed_term_ids": []string{term.ID},
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/complete-signup", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("complete signup: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify user created in DB
	u, found, err := userRepo.FindByKakaoID(ctx, 77777)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected user to be created in DB")
	}
	if u.Email != "new@user.com" {
		t.Fatalf("expected email new@user.com, got %s", u.Email)
	}

	// Verify consents saved
	consents, err := termsRepo.GetUserConsents(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(consents) != 1 || consents[0].TermID != term.ID {
		t.Fatalf("expected 1 consent for term %s, got %v", term.ID, consents)
	}

	// Verify JWT cookie set
	cookies := w.Result().Cookies()
	var tokenFound bool
	for _, c := range cookies {
		if c.Name == "ota_token" && c.Value != "" {
			tokenFound = true
		}
	}
	if !tokenFound {
		t.Fatal("expected ota_token cookie")
	}

	// Verify cache is busted
	if _, ok := signupCache.Get("test-signup-key"); ok {
		t.Fatal("expected signup cache entry to be deleted")
	}

	// Second attempt with same key should fail
	body2, _ := json.Marshal(map[string]any{
		"signup_key":      "test-signup-key",
		"agreed_term_ids": []string{term.ID},
	})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/auth/complete-signup", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusBadRequest {
		t.Fatalf("second attempt: expected 400, got %d", w2.Code)
	}
}
