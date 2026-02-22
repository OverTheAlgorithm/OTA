package collector

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func makeItem(sources ...string) ContextItem {
	return ContextItem{
		ID:       uuid.New(),
		Topic:    "테스트 주제",
		Summary:  "테스트 요약",
		Category: "top",
		Sources:  sources,
	}
}

func TestValidateSources_AllValid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html><body>Hello</body></html>")
	}))
	defer srv.Close()

	v := NewSourceValidator()
	items := []ContextItem{makeItem(srv.URL + "/article1", srv.URL + "/article2")}

	invalid := v.ValidateSources(context.Background(), items)

	if len(invalid) != 0 {
		t.Errorf("expected 0 invalid, got %d: %+v", len(invalid), invalid)
	}
}

func TestValidateSources_HTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	v := NewSourceValidator()
	items := []ContextItem{makeItem(srv.URL + "/missing")}

	invalid := v.ValidateSources(context.Background(), items)

	if len(invalid) != 1 {
		t.Fatalf("expected 1 invalid, got %d", len(invalid))
	}
	if invalid[0].Reason != "http 404" {
		t.Errorf("expected reason 'http 404', got %q", invalid[0].Reason)
	}
}

func TestValidateSources_SoftNotFoundKorean(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html><body>죄송합니다. 페이지를 찾을 수 없습니다.</body></html>")
	}))
	defer srv.Close()

	v := NewSourceValidator()
	items := []ContextItem{makeItem(srv.URL + "/soft-404")}

	invalid := v.ValidateSources(context.Background(), items)

	if len(invalid) != 1 {
		t.Fatalf("expected 1 invalid, got %d", len(invalid))
	}
	if invalid[0].ItemIndex != 0 {
		t.Errorf("expected item index 0, got %d", invalid[0].ItemIndex)
	}
}

func TestValidateSources_SoftNotFoundEnglish(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html><body><h1>Page Not Found</h1></body></html>")
	}))
	defer srv.Close()

	v := NewSourceValidator()
	items := []ContextItem{makeItem(srv.URL + "/not-here")}

	invalid := v.ValidateSources(context.Background(), items)

	if len(invalid) != 1 {
		t.Fatalf("expected 1 invalid, got %d", len(invalid))
	}
}

func TestValidateSources_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second) // exceeds 10s timeout
	}))
	defer srv.Close()

	v := NewSourceValidator()
	// Override with a shorter timeout for the test
	v.client.Timeout = 500 * time.Millisecond
	items := []ContextItem{makeItem(srv.URL + "/slow")}

	invalid := v.ValidateSources(context.Background(), items)

	if len(invalid) != 1 {
		t.Fatalf("expected 1 invalid, got %d", len(invalid))
	}
}

func TestValidateSources_InvalidScheme(t *testing.T) {
	v := NewSourceValidator()
	items := []ContextItem{makeItem("ftp://example.com/file")}

	invalid := v.ValidateSources(context.Background(), items)

	if len(invalid) != 1 {
		t.Fatalf("expected 1 invalid, got %d", len(invalid))
	}
	if invalid[0].Reason != "invalid scheme" {
		t.Errorf("expected reason 'invalid scheme', got %q", invalid[0].Reason)
	}
}

func TestValidateSources_EmptySources(t *testing.T) {
	v := NewSourceValidator()
	items := []ContextItem{makeItem()}

	invalid := v.ValidateSources(context.Background(), items)

	if len(invalid) != 0 {
		t.Errorf("expected 0 invalid for empty sources, got %d", len(invalid))
	}
}

func TestValidateSources_MixedResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/good" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "<html><body>Content</body></html>")
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	v := NewSourceValidator()
	items := []ContextItem{
		makeItem(srv.URL+"/good", srv.URL+"/bad"),
		makeItem(srv.URL + "/good"),
	}

	invalid := v.ValidateSources(context.Background(), items)

	if len(invalid) != 1 {
		t.Fatalf("expected 1 invalid, got %d: %+v", len(invalid), invalid)
	}
	if invalid[0].URL != srv.URL+"/bad" {
		t.Errorf("expected bad URL, got %q", invalid[0].URL)
	}
}

func TestValidateSources_MultipleNotFoundPatterns(t *testing.T) {
	patterns := []string{
		"존재하지 않는 페이지입니다",
		"삭제된 페이지입니다",
		"해당 페이지가 없습니다",
		"This page doesn't exist",
	}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				fmt.Fprintf(w, "<html><body>%s</body></html>", pattern)
			}))
			defer srv.Close()

			v := NewSourceValidator()
			items := []ContextItem{makeItem(srv.URL + "/page")}

			invalid := v.ValidateSources(context.Background(), items)

			if len(invalid) != 1 {
				t.Errorf("pattern %q: expected 1 invalid, got %d", pattern, len(invalid))
			}
		})
	}
}

func TestValidateSources_BlockedPortalURLs(t *testing.T) {
	blockedURLs := []string{
		"https://trends.google.co.kr/trending?geo=KR",
		"https://naver.com",
		"https://www.naver.com",
		"https://www.google.com",
		"https://daum.net",
		"https://finance.naver.com",
		"https://finance.naver.com/",
	}

	for _, u := range blockedURLs {
		t.Run(u, func(t *testing.T) {
			v := NewSourceValidator()
			items := []ContextItem{makeItem(u)}

			invalid := v.ValidateSources(context.Background(), items)

			if len(invalid) != 1 {
				t.Errorf("expected %q to be blocked, got %d invalid", u, len(invalid))
			}
		})
	}
}

func TestValidateSources_AllowedSpecificURLs(t *testing.T) {
	// These URLs point to specific content pages, not portal homepages.
	// We use a test server to avoid real HTTP calls.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<html><body>Article content</body></html>")
	}))
	defer srv.Close()

	v := NewSourceValidator()
	items := []ContextItem{makeItem(srv.URL + "/article/12345")}

	invalid := v.ValidateSources(context.Background(), items)

	if len(invalid) != 0 {
		t.Errorf("expected specific URL to be allowed, got %d invalid: %+v", len(invalid), invalid)
	}
}

func TestCheckBlockedURL_FinanceNaverWithPath(t *testing.T) {
	// finance.naver.com with a deep article path should be allowed
	reason := checkBlockedURL("https://finance.naver.com/item/main.naver?code=005930")
	if reason != "" {
		t.Errorf("expected finance.naver.com with article path to be allowed, got: %s", reason)
	}

	// finance.naver.com root should be blocked
	reason = checkBlockedURL("https://finance.naver.com")
	if reason == "" {
		t.Error("expected finance.naver.com root to be blocked")
	}
}
