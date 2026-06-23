package communities

import (
	"context"
	"io"
	"net/http"
	"time"
)

// RobotsHTTPFetcher implements communitytrend.RobotsFetcher over HTTP.
// Fetching robots.txt itself is universally permitted (the file exists to be
// read); the allowance decision is made by the caller from the body.
type RobotsHTTPFetcher struct{}

func NewRobotsHTTPFetcher() RobotsHTTPFetcher { return RobotsHTTPFetcher{} }

// Fetch returns the robots.txt body and an accessible flag.
//   - 200: body returned, accessible=true (caller parses rules)
//   - 404: empty body, accessible=true (no robots = default allow)
//   - 403/429/timeout/network error: accessible=false (anti-bot wall → treat as disallowed, decisions.md D-006)
//   - other non-2xx: accessible=false (be conservative)
func (RobotsHTTPFetcher) Fetch(ctx context.Context, robotsURL string) (string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := politeClient.Do(req)
	if err != nil {
		return "", false, err // network/timeout → not accessible
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB
		if err != nil {
			return "", false, err
		}
		return string(body), true, nil
	case resp.StatusCode == http.StatusNotFound:
		return "", true, nil // no robots.txt = allowed by default
	default:
		return "", false, nil // 403/429/5xx → conservatively disallowed
	}
}
