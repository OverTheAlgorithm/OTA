package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ota/api"
	"ota/api/handler"
	"ota/auth"
	"ota/domain/collector"
	"ota/platform/kakao"
	"ota/storage"
)

func TestAPI_AdminCollectEndpoint(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	// Setup services
	aiClient := &mockAIClient{
		resp: collector.AIResponse{
			OutputText: validJSON,
			RawJSON:    `{"raw":"data"}`,
		},
	}
	collectorRepo := storage.NewCollectorRepository(db.Pool)
	collectorService := collector.NewService(aiClient, collectorRepo)

	adminHandler := handler.NewAdminHandler(collectorService, "") // no Slack webhook in tests

	// Setup router
	gin.SetMode(gin.TestMode)
	testJWT := auth.NewJWTManager("test-secret")
	router := api.NewRouter("api", "v1", "http://localhost:5173", testJWT, []api.RouteModule{
		{
			GroupName:   "admin",
			Handler:     adminHandler,
			Middlewares: []gin.HandlerFunc{},
		},
	})

	// Endpoint is async: returns 202 immediately, runs collection in background.
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/admin/collect", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response["message"] != "collection started" {
		t.Errorf("unexpected message: %v", response["message"])
	}

	// Second call should also return 202 immediately
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/admin/collect", nil)
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusAccepted {
		t.Errorf("expected second call to return 202, got %d", w2.Code)
	}
}

func TestAPI_AuthFlow(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "users")

	// Setup user repository
	userRepo := storage.NewUserRepository(db.Pool)
	jwtManager := auth.NewJWTManager("test-secret-key-for-integration-tests")

	// Create a test user
	testUser, err := userRepo.UpsertByKakaoID(
		context.Background(),
		12345,
		"test@example.com",
		"테스트유저",
		"https://example.com/profile.jpg",
	)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	// Generate JWT token
	token, err := jwtManager.Generate(testUser.ID, testUser.Role)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	// Create handlers (Kakao won't be called in these tests, but needed for initialization)
	kakaoClient := kakao.NewClient("dummy-id", "dummy-secret", "http://localhost/callback")
	stateStore := auth.NewStateStore()
	authHandler := handler.NewAuthHandler(kakaoClient, jwtManager, stateStore, userRepo, nil, "http://localhost:5173")

	gin.SetMode(gin.TestMode)
	router := api.NewRouter("api", "v1", "http://localhost:5173", jwtManager, []api.RouteModule{
		{
			GroupName:   "auth",
			Handler:     authHandler,
			Middlewares: []gin.HandlerFunc{},
		},
	})

	// Test /me endpoint with valid token
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "ota_token", Value: token})
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	data, ok := response["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data field")
	}

	if data["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", data["email"])
	}

	// Test /me endpoint without token
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/auth/me", nil)
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 without token, got %d", w2.Code)
	}

	// Test logout
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("POST", "/api/v1/auth/logout", nil)
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("expected status 200 for logout, got %d", w3.Code)
	}
}

func TestAPI_CORSMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testJWT := auth.NewJWTManager("test-secret")
	router := api.NewRouter("api", "v1", "http://localhost:5173", testJWT, []api.RouteModule{})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/api/v1/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	router.ServeHTTP(w, req)

	// CORS headers should be present
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Errorf("expected CORS header, got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}
}
