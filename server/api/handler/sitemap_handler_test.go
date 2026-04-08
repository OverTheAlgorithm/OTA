package handler_test

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"ota/api/handler"
)

// ─── Mock ───────────────────────────────────────────────────────────────────

type mockSitemapRepoForHandler struct {
	topics []handler.TopicEntry
	err    error
}

func (m *mockSitemapRepoForHandler) GetAllTopicIDs(_ context.Context) ([]handler.TopicEntry, error) {
	return m.topics, m.err
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestSitemapHandler_ContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := handler.NewSitemapHandler(&mockSitemapRepoForHandler{}, "https://wizletter.mindhacker.club")
	r := gin.New()
	group := r.Group("/api/v1")
	h.RegisterRoutes(group)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sitemap.xml", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/xml") {
		t.Fatalf("expected application/xml content-type, got %q", ct)
	}
}

func TestSitemapHandler_StaticPages(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := handler.NewSitemapHandler(&mockSitemapRepoForHandler{}, "https://wizletter.mindhacker.club")
	r := gin.New()
	group := r.Group("/api/v1")
	h.RegisterRoutes(group)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sitemap.xml", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	staticURLs := []string{
		"https://wizletter.mindhacker.club/",
		"https://wizletter.mindhacker.club/latest",
		"https://wizletter.mindhacker.club/allnews",
		"https://wizletter.mindhacker.club/privacy-policy",
		"https://wizletter.mindhacker.club/terms-of-service",
		"https://wizletter.mindhacker.club/cookie-policy",
		"https://wizletter.mindhacker.club/about",
	}
	for _, u := range staticURLs {
		if !strings.Contains(body, u) {
			t.Errorf("expected sitemap to contain %q", u)
		}
	}
}

func TestSitemapHandler_TopicURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fixedTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	repo := &mockSitemapRepoForHandler{
		topics: []handler.TopicEntry{
			{ID: "123", CreatedAt: fixedTime},
			{ID: "456", CreatedAt: fixedTime.Add(-24 * time.Hour)},
		},
	}
	h := handler.NewSitemapHandler(repo, "https://wizletter.mindhacker.club")
	r := gin.New()
	group := r.Group("/api/v1")
	h.RegisterRoutes(group)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sitemap.xml", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "https://wizletter.mindhacker.club/topic/123") {
		t.Error("expected sitemap to contain topic/123")
	}
	if !strings.Contains(body, "https://wizletter.mindhacker.club/topic/456") {
		t.Error("expected sitemap to contain topic/456")
	}
}

func TestSitemapHandler_ValidXML(t *testing.T) {
	gin.SetMode(gin.TestMode)

	fixedTime := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	repo := &mockSitemapRepoForHandler{
		topics: []handler.TopicEntry{
			{ID: "999", CreatedAt: fixedTime},
		},
	}
	h := handler.NewSitemapHandler(repo, "https://wizletter.mindhacker.club")
	r := gin.New()
	group := r.Group("/api/v1")
	h.RegisterRoutes(group)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sitemap.xml", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var urlSet struct {
		XMLName xml.Name `xml:"urlset"`
		URLs    []struct {
			Loc        string `xml:"loc"`
			LastMod    string `xml:"lastmod"`
			ChangeFreq string `xml:"changefreq"`
			Priority   string `xml:"priority"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &urlSet); err != nil {
		t.Fatalf("response is not valid XML: %v", err)
	}

	// 7 static + 1 topic
	if len(urlSet.URLs) != 8 {
		t.Fatalf("expected 8 URL entries, got %d", len(urlSet.URLs))
	}
}

func TestSitemapHandler_RepoError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &mockSitemapRepoForHandler{err: context.DeadlineExceeded}
	h := handler.NewSitemapHandler(repo, "https://wizletter.mindhacker.club")
	r := gin.New()
	group := r.Group("/api/v1")
	h.RegisterRoutes(group)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sitemap.xml", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on repo error, got %d", w.Code)
	}
}
