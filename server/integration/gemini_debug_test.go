package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"ota/platform/gemini"
)

// TestGemini_Debug helps debug Gemini API issues
func TestGemini_Debug(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}

	t.Logf("API Key (first 10 chars): %s...", apiKey[:min(10, len(apiKey))])

	// Test available models (Feb 2026 - Gemini 1.x retired)
	models := []string{
		"gemini-2.0-flash",
		"gemini-2.5-flash-lite",
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			client := gemini.NewClient(apiKey, model)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Simple prompt without complex JSON
			prompt := "What is 2+2? Answer in one sentence."

			resp, err := client.SearchAndAnalyze(ctx, prompt)
			if err != nil {
				t.Logf("Model %s failed: %v", model, err)
				return
			}

			t.Logf("Model %s SUCCESS!", model)
			t.Logf("  Output: %s", resp.OutputText[:min(100, len(resp.OutputText))])
			t.Logf("  Annotations: %d", len(resp.Annotations))
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
