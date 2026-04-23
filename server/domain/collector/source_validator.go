package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// InvalidSource describes a source URL that failed validation.
type InvalidSource struct {
	ItemIndex int
	URL       string
	Reason    string
}

// SourceValidator checks whether source URLs actually exist
// by making HTTP GET requests and inspecting responses.
type SourceValidator struct {
	client *http.Client
}

func NewSourceValidator() *SourceValidator {
	return newSourceValidatorWithTransport(newSafeTransport())
}

// newSourceValidatorWithTransport creates a SourceValidator using the provided
// transport. Intended for tests that need to reach httptest servers on loopback.
func newSourceValidatorWithTransport(transport http.RoundTripper) *SourceValidator {
	return &SourceValidator{
		client: &http.Client{
			Timeout:       10 * time.Second,
			Transport:     transport,
			CheckRedirect: safeRedirectPolicy,
		},
	}
}

// notFoundPatterns are phrases commonly found on 404/deleted pages in Korean and English.
var notFoundPatterns = []string{
	"페이지를 찾을 수 없습니다",
	"존재하지 않는 페이지",
	"요청하신 페이지를 찾을 수 없",
	"존재하지 않는 링크",
	"찾을 수 없는 페이지",
	"삭제된 페이지",
	"삭제된 기사",
	"해당 페이지가 없습니다",
	"페이지가 존재하지 않",
	"기사가 존재하지 않",
	"삭제되었거나 존재하지 않는",
	"서비스가 종료",
	"잘못된 접근",
	"더 이상 제공되지 않",
	"잘못된 웹 주소로",
	"404 not found",
	"page not found",
	"not found",
	"this page doesn't exist",
	"this page does not exist",
	"no longer available",
	"article not found",
	"content not found",
}

// blockedHosts are portal/aggregator sites that are never valid as a topic-specific source.
// These URLs point to homepages or search pages, not to specific topic content.
var blockedHosts = []string{
	"trends.google.co.kr",
	"trends.google.com",
	"naver.com",
	"www.naver.com",
	"m.naver.com",
	"daum.net",
	"www.daum.net",
	"m.daum.net",
	"google.com",
	"www.google.com",
	"google.co.kr",
	"www.google.co.kr",
	"news.google.com", // Google News redirect URLs that failed Stage 2 decoding
}

// blockedPathPrefixes are host+path combos that are too generic (e.g. finance.naver.com with no article path).
var blockedPathPrefixes = []struct {
	host    string
	maxPath int // if path segment count <= this, block it (e.g. "/" = 0 segments, "/item" = 1)
}{
	{"finance.naver.com", 1},
	{"search.naver.com", 1},
	{"search.daum.net", 1},
	{"m.search.naver.com", 1},
}

const (
	maxConcurrentChecks = 10
	maxBodyRead         = 100 * 1024 // 100KB — Korean sites often have 50KB+ of CSS/JS in <head>
)

// ValidateSources checks all source URLs across all items concurrently.
// Returns a list of invalid sources. Does not modify the input items.
func (v *SourceValidator) ValidateSources(ctx context.Context, items []ContextItem) []InvalidSource {
	type checkJob struct {
		itemIndex int
		url       string
	}

	var jobs []checkJob
	for i, item := range items {
		for _, u := range item.Sources {
			if strings.TrimSpace(u) == "" {
				continue
			}
			jobs = append(jobs, checkJob{itemIndex: i, url: u})
		}
	}

	if len(jobs) == 0 {
		return nil
	}

	var (
		mu      sync.Mutex
		invalid []InvalidSource
		wg      sync.WaitGroup
		sem     = make(chan struct{}, maxConcurrentChecks)
	)

	for _, job := range jobs {
		wg.Add(1)
		go func(j checkJob) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			reason := v.checkURL(ctx, j.url)
			if reason != "" {
				mu.Lock()
				invalid = append(invalid, InvalidSource{
					ItemIndex: j.itemIndex,
					URL:       j.url,
					Reason:    reason,
				})
				mu.Unlock()
			}
		}(job)
	}

	wg.Wait()
	return invalid
}

// checkURL performs a single URL check. Returns empty string if valid,
// or a reason string if invalid.
func (v *SourceValidator) checkURL(ctx context.Context, rawURL string) string {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return "invalid scheme"
	}

	// Block portal/aggregator homepages that aren't topic-specific.
	if reason := checkBlockedURL(rawURL); reason != "" {
		return reason
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Sprintf("invalid url: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Sprintf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Sprintf("http %d", resp.StatusCode)
	}

	// Read a small portion of the body to detect soft-404 pages.
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyRead))
	if err != nil {
		// Body read failure on a 2xx/3xx is suspicious but not conclusive.
		return ""
	}

	bodyLower := strings.ToLower(string(bodyBytes))
	for _, pattern := range notFoundPatterns {
		if strings.Contains(bodyLower, strings.ToLower(pattern)) {
			return fmt.Sprintf("page contains not-found text: %q", pattern)
		}
	}

	return ""
}

// checkBlockedURL returns a reason if the URL is a portal/aggregator homepage
// that doesn't point to specific topic content. Returns "" if allowed.
func checkBlockedURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())

	// Exact host block (portal homepages, trend aggregators).
	for _, blocked := range blockedHosts {
		if host == blocked {
			return fmt.Sprintf("blocked portal/aggregator: %s", host)
		}
	}

	// Path-depth block (e.g. finance.naver.com with no article path).
	pathSegments := countPathSegments(parsed.Path)
	for _, bp := range blockedPathPrefixes {
		if host == bp.host && pathSegments <= bp.maxPath {
			return fmt.Sprintf("blocked generic page: %s (path too short)", host)
		}
	}

	return ""
}

// countPathSegments counts non-empty segments in a URL path.
// "/" → 0, "/finance" → 1, "/finance/article/123" → 3
func countPathSegments(path string) int {
	count := 0
	for _, seg := range strings.Split(path, "/") {
		if seg != "" {
			count++
		}
	}
	return count
}
