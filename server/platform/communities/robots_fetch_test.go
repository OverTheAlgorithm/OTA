package communities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRobotsHTTPFetcher_StatusHandling(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		body           string
		wantAccessible bool
		wantBody       string
	}{
		{"200 with body", http.StatusOK, "User-agent: *\nAllow: /\n", true, "User-agent: *\nAllow: /\n"},
		{"404 no robots", http.StatusNotFound, "", true, ""},
		{"403 wall", http.StatusForbidden, "", false, ""},
		{"429 rate limited", http.StatusTooManyRequests, "", false, ""},
		{"500 server error", http.StatusInternalServerError, "", false, ""},
	}
	f := NewRobotsHTTPFetcher()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			body, accessible, err := f.Fetch(context.Background(), srv.URL+"/robots.txt")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if accessible != tc.wantAccessible {
				t.Fatalf("accessible=%v want %v", accessible, tc.wantAccessible)
			}
			if body != tc.wantBody {
				t.Fatalf("body=%q want %q", body, tc.wantBody)
			}
		})
	}
}
