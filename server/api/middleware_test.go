package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"ota/auth"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRateLimitRouter(ratePerMin int, jwtManager *auth.JWTManager) *gin.Engine {
	r := gin.New()
	r.Use(RateLimitMiddleware(ratePerMin, jwtManager))
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	return r
}

func TestRateLimitMiddleware_AllowsUnderLimit(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-secret")
	router := setupRateLimitRouter(5, jwtManager)

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", nil)
		req.RemoteAddr = "1.2.3.4:12345"
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimitMiddleware_BlocksOverLimit(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-secret")
	router := setupRateLimitRouter(3, jwtManager)

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", nil)
		req.RemoteAddr = "1.2.3.4:12345"
		router.ServeHTTP(w, req)
	}

	// 4th request should be blocked
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	req.RemoteAddr = "1.2.3.4:12345"
	router.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_AuthenticatedUserSameBucket(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-secret")
	token, err := jwtManager.Generate("user-123", "user")
	if err != nil {
		t.Fatal(err)
	}

	router := setupRateLimitRouter(2, jwtManager)

	// Request from IP A with token
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/ping", nil)
	req1.RemoteAddr = "10.0.0.1:1111"
	req1.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("request 1: expected 200, got %d", w1.Code)
	}

	// Request from IP B with same token
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/ping", nil)
	req2.RemoteAddr = "10.0.0.2:2222"
	req2.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("request 2: expected 200, got %d", w2.Code)
	}

	// 3rd request (same user, different IP) should be blocked
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/ping", nil)
	req3.RemoteAddr = "10.0.0.3:3333"
	req3.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w3, req3)

	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("request 3: expected 429, got %d", w3.Code)
	}
}

func TestRateLimitMiddleware_DifferentIPsSeparateBuckets(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-secret")
	router := setupRateLimitRouter(2, jwtManager)

	// 2 requests from IP A — should be OK
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/ping", nil)
		req.RemoteAddr = "10.0.0.1:1111"
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("IP-A request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// IP A is now exhausted
	wBlocked := httptest.NewRecorder()
	reqBlocked, _ := http.NewRequest("GET", "/ping", nil)
	reqBlocked.RemoteAddr = "10.0.0.1:1111"
	router.ServeHTTP(wBlocked, reqBlocked)

	if wBlocked.Code != http.StatusTooManyRequests {
		t.Fatalf("IP-A 3rd request: expected 429, got %d", wBlocked.Code)
	}

	// IP B should still be allowed
	wOther := httptest.NewRecorder()
	reqOther, _ := http.NewRequest("GET", "/ping", nil)
	reqOther.RemoteAddr = "10.0.0.2:2222"
	router.ServeHTTP(wOther, reqOther)

	if wOther.Code != http.StatusOK {
		t.Fatalf("IP-B request: expected 200, got %d", wOther.Code)
	}
}
