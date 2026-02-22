package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
	return &SourceValidator{
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

// notFoundPatterns are phrases commonly found on 404/deleted pages in Korean and English.
var notFoundPatterns = []string{
	"페이지를 찾을 수 없습니다",
	"존재하지 않는 페이지",
	"요청하신 페이지를 찾을 수 없",
	"찾을 수 없는 페이지",
	"삭제된 페이지",
	"해당 페이지가 없습니다",
	"페이지가 존재하지 않",
	"404 not found",
	"page not found",
	"not found",
	"this page doesn't exist",
	"this page does not exist",
}

const (
	maxConcurrentChecks = 10
	maxBodyRead         = 10 * 1024 // 10KB
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Sprintf("invalid url: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; OTA-Bot/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*")

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
