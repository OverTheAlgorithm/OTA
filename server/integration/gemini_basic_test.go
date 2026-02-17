package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestGemini_BasicCall tests Gemini WITHOUT grounding to verify basic API works
func TestGemini_BasicCall(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}

	models := []string{
		"gemini-2.0-flash",
		"gemini-2.5-flash-lite",
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			// Request WITHOUT google_search tool
			reqBody := map[string]interface{}{
				"contents": []map[string]interface{}{
					{
						"parts": []map[string]string{
							{"text": "What is 2+2? Answer in one sentence."},
						},
					},
				},
			}

			jsonBody, _ := json.Marshal(reqBody)
			url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)

			req, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-goog-api-key", apiKey)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			req = req.WithContext(ctx)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != 200 {
				t.Logf("Model %s FAILED (status %d): %s", model, resp.StatusCode, string(body))
				return
			}

			t.Logf("Model %s SUCCESS!", model)
			t.Logf("Response preview: %s", string(body)[:min(200, len(body))])
		})
	}
}

// TestGemini_WithGrounding tests Gemini WITH google_search tool
func TestGemini_WithGrounding(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}

	models := []string{
		"gemini-2.0-flash",
		"gemini-2.5-flash-lite",
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			// Request WITH google_search tool
			reqBody := map[string]interface{}{
				"contents": []map[string]interface{}{
					{
						"parts": []map[string]string{
							{"text": "What are the latest news in South Korea today?"},
						},
					},
				},
				"tools": []map[string]interface{}{
					{"google_search": map[string]interface{}{}},
				},
			}

			jsonBody, _ := json.Marshal(reqBody)
			url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)

			req, _ := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-goog-api-key", apiKey)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			req = req.WithContext(ctx)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != 200 {
				t.Logf("Model %s with grounding FAILED (status %d): %s", model, resp.StatusCode, string(body))
				return
			}

			t.Logf("Model %s with grounding SUCCESS!", model)
			t.Logf("Response preview: %s", string(body)[:min(500, len(body))])
		})
	}
}
