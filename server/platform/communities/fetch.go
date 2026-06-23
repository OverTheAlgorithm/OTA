package communities

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// UserAgent identifies our bot honestly (legal guardrail: no disguise).
const UserAgent = "WizLetterBot/0.1 (+https://wizletter.com/bot; community trend research)"

// politeClient is a shared HTTP client with a conservative timeout.
var politeClient = &http.Client{Timeout: 20 * time.Second}

// fetchHTML performs a single GET with our honest UA and returns the body bytes.
// Callers are responsible for rate limiting (one site at a time, once per day).
func fetchHTML(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("new request %s: %w", url, err)
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "ko-KR,ko;q=0.9")

	resp, err := politeClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8 MiB cap
	if err != nil {
		return nil, fmt.Errorf("read body %s: %w", url, err)
	}
	return body, nil
}
