package e2e

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"ota/domain/collector"
	"ota/platform/gemini"
	"ota/platform/openai"
	"ota/storage"
)

// TestAIClient_OpenAI tests the actual OpenAI client with real API calls.
// This test is skipped unless OPENAI_API_KEY is set.
// Run with: OPENAI_API_KEY=sk-... go test -v -run TestAIClient_OpenAI
func TestAIClient_OpenAI(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping OpenAI integration test")
	}

	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-4o" // default model
	}

	client := openai.NewClient(apiKey, model)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	prompt := `Please analyze current trending topics in South Korea and return a JSON response with the following structure:
{
  "items": [
    {
      "category": "top",
      "rank": 1,
      "topic": "topic title",
      "summary": "brief summary",
      "sources": ["url1", "url2"]
    }
  ]
}

Find 3-5 trending topics across different categories (top, entertainment, economy, etc.).`

	resp, err := client.SearchAndAnalyze(ctx, prompt)
	if err != nil {
		t.Fatalf("OpenAI client failed: %v", err)
	}

	// Verify response structure
	if resp.OutputText == "" {
		t.Error("expected non-empty output text")
	}

	if resp.RawJSON == "" {
		t.Error("expected non-empty raw JSON")
	}

	t.Logf("OpenAI response received:")
	t.Logf("  Output length: %d chars", len(resp.OutputText))
	t.Logf("  Annotations: %d", len(resp.Annotations))
	t.Logf("  Raw JSON length: %d chars", len(resp.RawJSON))

	// Try to parse as context items
	items, err := parseContextItemsFromResponse(resp.OutputText)
	if err != nil {
		t.Logf("Warning: Failed to parse as context items: %v", err)
		t.Logf("Output text: %s", resp.OutputText)
	} else {
		t.Logf("Successfully parsed %d context items", len(items))
		for i, item := range items {
			t.Logf("  Item %d: [%s] %s", i+1, item.Category, item.Topic)
		}
	}
}

// TestAIClient_Gemini tests the actual Gemini client with real API calls.
// This test is skipped unless GEMINI_API_KEY is set.
// Run with: GEMINI_API_KEY=... go test -v -run TestAIClient_Gemini
func TestAIClient_Gemini(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping Gemini integration test")
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.5-flash-lite" // current stable model
	}

	client := gemini.NewClient(apiKey, model)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	prompt := `Please analyze current trending topics in South Korea and return a JSON response with the following structure:
{
  "items": [
    {
      "category": "top",
      "rank": 1,
      "topic": "topic title",
      "summary": "brief summary",
      "sources": ["url1", "url2"]
    }
  ]
}

Find 3-5 trending topics across different categories (top, entertainment, economy, etc.).`

	resp, err := client.SearchAndAnalyze(ctx, prompt)
	if err != nil {
		t.Fatalf("Gemini client failed: %v", err)
	}

	// Verify response structure
	if resp.OutputText == "" {
		t.Error("expected non-empty output text")
	}

	if resp.RawJSON == "" {
		t.Error("expected non-empty raw JSON")
	}

	t.Logf("Gemini response received:")
	t.Logf("  Output length: %d chars", len(resp.OutputText))
	t.Logf("  Annotations: %d", len(resp.Annotations))
	t.Logf("  Raw JSON length: %d chars", len(resp.RawJSON))

	// Try to parse as context items
	items, err := parseContextItemsFromResponse(resp.OutputText)
	if err != nil {
		t.Logf("Warning: Failed to parse as context items: %v", err)
		t.Logf("Output text: %s", resp.OutputText)
	} else {
		t.Logf("Successfully parsed %d context items", len(items))
		for i, item := range items {
			t.Logf("  Item %d: [%s] %s", i+1, item.Category, item.Topic)
		}
	}
}

// TestAIClient_RetryLogic tests the retry mechanism with a failing client.
func TestAIClient_RetryLogic(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	// Create a client that always fails with a retryable error
	failingClient := &failingAIClient{
		failCount: 2, // Fail first 2 attempts, succeed on 3rd
		err:       &collector.AIError{Type: collector.ErrorTypeNetwork, Message: "simulated network error"},
	}

	repo := storage.NewCollectorRepository(db.Pool)
	sc := &e2eSourceCollector{items: []collector.TrendingItem{
		{Keyword: "test", Source: "test", Traffic: 100},
	}}
	agg := collector.NewAggregator([]collector.SourceCollector{sc})
	service := collector.NewService(failingClient, repo)
	service.WithAggregator(agg)

	result, err := service.CollectFromSources(context.Background())
	if err != nil {
		t.Fatalf("expected success after retries, got error: %v", err)
	}

	if len(result.Items) == 0 {
		t.Error("expected items in result")
	}

	if failingClient.attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", failingClient.attempts)
	}
}

// TestAIClient_NoRetryOnFormatError tests that format errors don't trigger retries.
func TestAIClient_NoRetryOnFormatError(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Truncate(t, "context_items", "collection_runs")

	// Create a client that always fails with a non-retryable error
	failingClient := &failingAIClient{
		failCount: 10, // Always fail
		err:       &collector.AIError{Type: collector.ErrorTypeFormat, Message: "invalid response format"},
	}

	repo := storage.NewCollectorRepository(db.Pool)
	sc := &e2eSourceCollector{items: []collector.TrendingItem{
		{Keyword: "test", Source: "test", Traffic: 100},
	}}
	agg := collector.NewAggregator([]collector.SourceCollector{sc})
	service := collector.NewService(failingClient, repo)
	service.WithAggregator(agg)

	_, err := service.CollectFromSources(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should only attempt once (no retries for format errors)
	if failingClient.attempts != 1 {
		t.Errorf("expected 1 attempt for format error, got %d", failingClient.attempts)
	}
}

// TestAIClient_OpenAI_InvalidAPIKey verifies that invalid API key errors are properly classified.
// This always runs (no API key needed) to verify authentication error handling.
func TestAIClient_OpenAI_InvalidAPIKey(t *testing.T) {
	client := openai.NewClient("sk-invalid-key-for-testing", "gpt-4o")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prompt := "Test prompt"

	_, err := client.SearchAndAnalyze(ctx, prompt)
	if err == nil {
		t.Fatal("expected authentication error with invalid API key, got nil")
	}

	t.Logf("Error received: %v", err)

	// Classify the error
	aiErr := collector.ClassifyError(err)
	if aiErr == nil {
		t.Fatal("expected AIError, got nil")
	}

	t.Logf("Classified as: %s", aiErr.Type)

	// Authentication errors should be classified as Infrastructure errors (401/403 status codes)
	if aiErr.Type != collector.ErrorTypeInfrastructure {
		t.Errorf("expected InfrastructureError for auth failure, got %s", aiErr.Type)
	}

	// Verify it's retryable (infrastructure errors should retry in case of temporary issues)
	if !aiErr.IsRetryable() {
		t.Error("expected authentication error to be retryable (infrastructure type)")
	}

	// Verify the error message contains authentication-related keywords
	errMsg := err.Error()
	hasAuthKeyword := containsAny(errMsg, "401", "403", "unauthorized", "authentication", "invalid", "api key")
	if !hasAuthKeyword {
		t.Logf("Warning: Error message doesn't contain obvious authentication keywords: %s", errMsg)
	}
}

// TestAIClient_Gemini_InvalidAPIKey verifies that invalid API key errors are properly classified.
func TestAIClient_Gemini_InvalidAPIKey(t *testing.T) {
	client := gemini.NewClient("invalid-api-key-for-testing", "gemini-1.5-flash-002")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prompt := "Test prompt"

	_, err := client.SearchAndAnalyze(ctx, prompt)
	if err == nil {
		t.Fatal("expected authentication error with invalid API key, got nil")
	}

	t.Logf("Error received: %v", err)

	// Classify the error
	aiErr := collector.ClassifyError(err)
	if aiErr == nil {
		t.Fatal("expected AIError, got nil")
	}

	t.Logf("Classified as: %s", aiErr.Type)

	// Authentication errors should be classified as Infrastructure errors
	if aiErr.Type != collector.ErrorTypeInfrastructure {
		t.Errorf("expected InfrastructureError for auth failure, got %s", aiErr.Type)
	}

	// Verify the error message contains authentication-related keywords
	errMsg := err.Error()
	hasAuthKeyword := containsAny(errMsg, "400", "401", "403", "unauthorized", "authentication", "invalid", "api key", "API_KEY")
	if !hasAuthKeyword {
		t.Logf("Warning: Error message doesn't contain obvious authentication keywords: %s", errMsg)
	}
}

// e2eSourceCollector is a simple mock for SourceCollector in e2e tests.
type e2eSourceCollector struct {
	items []collector.TrendingItem
}

func (m *e2eSourceCollector) Name() string { return "e2e_test" }
func (m *e2eSourceCollector) Collect(_ context.Context) ([]collector.TrendingItem, error) {
	return m.items, nil
}

// Helper functions and types

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

type failingAIClient struct {
	attempts  int
	failCount int
	err       error
}

func (f *failingAIClient) SearchAndAnalyze(ctx context.Context, prompt string) (collector.AIResponse, error) {
	f.attempts++

	if f.attempts <= f.failCount {
		return collector.AIResponse{}, f.err
	}

	// Succeed after failCount attempts
	return collector.AIResponse{
		OutputText: validJSON,
		RawJSON:    `{"raw":"data"}`,
	}, nil
}

func parseContextItemsFromResponse(outputText string) ([]testContextItem, error) {
	// Use same parsing logic as service, but with local types for testing
	var payload struct {
		Items []testContextItem `json:"items"`
	}

	if err := json.Unmarshal([]byte(outputText), &payload); err != nil {
		return nil, err
	}

	return payload.Items, nil
}

type testContextItem struct {
	Category string   `json:"category"`
	Rank     int      `json:"rank"`
	Topic    string   `json:"topic"`
	Summary  string   `json:"summary"`
	Sources  []string `json:"sources"`
}

const validJSON = `{
	"items": [
		{
			"category": "top",
			"rank": 1,
			"topic": "테스트 주제 1",
			"summary": "첫 번째 테스트 요약",
			"sources": ["https://example1.com"]
		},
		{
			"category": "entertainment",
			"rank": 1,
			"topic": "테스트 주제 2",
			"summary": "두 번째 테스트 요약",
			"sources": ["https://example2.com"]
		},
		{
			"category": "economy",
			"rank": 1,
			"topic": "테스트 주제 3",
			"summary": "세 번째 테스트 요약",
			"sources": ["https://example3.com"]
		}
	]
}`
