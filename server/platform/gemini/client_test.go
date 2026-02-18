package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchAndAnalyze_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Error("missing api key header")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing content-type header")
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(validAPIResponse))
	}))
	defer server.Close()

	client := NewClient("test-key", "gemini-2.5-flash")
	client.httpClient = server.Client()
	client.httpClient.Transport = rewriteTransport{base: client.httpClient.Transport, url: server.URL}

	resp, err := client.SearchAndAnalyze(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.OutputText != `{"items":[{"category":"top","rank":1,"topic":"테스트","summary":"테스트 요약","sources":["https://example.com"]}]}` {
		t.Errorf("unexpected output text: %s", resp.OutputText)
	}
	if len(resp.Annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(resp.Annotations))
	}
	if resp.Annotations[0].URL != "https://example.com" {
		t.Errorf("unexpected annotation url: %s", resp.Annotations[0].URL)
	}
}

func TestSearchAndAnalyze_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	client := NewClient("test-key", "gemini-2.5-flash")
	client.httpClient = server.Client()
	client.httpClient.Transport = rewriteTransport{base: client.httpClient.Transport, url: server.URL}

	_, err := client.SearchAndAnalyze(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

func TestParseResponse_Success(t *testing.T) {
	resp, err := parseResponse([]byte(validAPIResponse))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.OutputText == "" {
		t.Error("output text should not be empty")
	}
	if len(resp.Annotations) != 1 {
		t.Errorf("expected 1 annotation, got %d", len(resp.Annotations))
	}
}

func TestParseResponse_NoCandidates(t *testing.T) {
	_, err := parseResponse([]byte(`{"candidates":[]}`))
	if err == nil {
		t.Fatal("expected error for empty candidates")
	}
}

func TestParseResponse_MalformedJSON(t *testing.T) {
	_, err := parseResponse([]byte(`{broken`))
	if err == nil {
		t.Fatal("expected error for malformed json")
	}
}

func TestParseResponse_GroundingChunks(t *testing.T) {
	resp, err := parseResponse([]byte(responseWithGroundingChunks))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Annotations) != 1 {
		t.Fatalf("expected 1 annotation from grounding chunks, got %d", len(resp.Annotations))
	}
	if resp.Annotations[0].URL != "https://chunk.example.com" {
		t.Errorf("unexpected annotation url: %s", resp.Annotations[0].URL)
	}
}

type rewriteTransport struct {
	base http.RoundTripper
	url  string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.url[len("http://"):]
	return t.base.RoundTrip(req)
}

const validAPIResponse = `{
	"candidates": [{
		"content": {
			"parts": [{"text": "{\"items\":[{\"category\":\"top\",\"rank\":1,\"topic\":\"테스트\",\"summary\":\"테스트 요약\",\"sources\":[\"https://example.com\"]}]}"}]
		},
		"groundingMetadata": {
			"searchResult": [{"title": "Example", "url": "https://example.com", "snippet": "test"}]
		}
	}]
}`

const responseWithGroundingChunks = `{
	"candidates": [{
		"content": {
			"parts": [{"text": "{\"items\":[{\"category\":\"top\",\"rank\":1,\"topic\":\"청크\",\"summary\":\"청크 테스트\",\"sources\":[]}]}"}]
		},
		"groundingMetadata": {
			"groundingChunks": [{"web": {"uri": "https://chunk.example.com", "title": "Chunk Source"}}]
		}
	}]
}`
