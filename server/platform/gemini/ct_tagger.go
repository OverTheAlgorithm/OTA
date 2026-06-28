package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"ota/domain/communitytrend"
)

// CTTagger implements communitytrend.Tagger using Gemini. Unlike the collector
// client it uses no Google Search tool and requests strict JSON output — it
// only classifies the provided titles into the supplied taxonomy.
type CTTagger struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewCTTagger(apiKey, model string) *CTTagger {
	return &CTTagger{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 90 * time.Second},
	}
}

func (t *CTTagger) Analyze(ctx context.Context, in communitytrend.TaggerInput) (communitytrend.TaggerOutput, error) {
	if len(in.Titles) == 0 {
		return communitytrend.TaggerOutput{}, nil
	}
	slog.Info("sending tagger request to Gemini", "community", in.CommunityKey, "titles_count", len(in.Titles), "titles", in.Titles)
	prompt := communitytrend.BuildTagPrompt(in)

	reqBody := ctRequest{
		Contents: []ctContent{{Parts: []ctPart{{Text: prompt}}}},
		GenerationConfig: &ctGenerationConfig{
			ResponseMimeType: "application/json",
			ThinkingConfig:   &thinkingConfig{ThinkingBudget: -1},
		},
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return communitytrend.TaggerOutput{}, fmt.Errorf("marshal tagger request: %w", err)
	}

	url := baseURL + t.model + ":generateContent"

	maxAttempts := 4
	baseDelay := 1 * time.Second

	var lastErr error
	var respBody []byte
	var statusCode int

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Respect context cancellation
		if err := ctx.Err(); err != nil {
			return communitytrend.TaggerOutput{}, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
		if err != nil {
			return communitytrend.TaggerOutput{}, fmt.Errorf("create tagger request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", t.apiKey)

		resp, err := t.httpClient.Do(req)
		if err != nil {
			lastErr = err
			statusCode = 0
		} else {
			statusCode = resp.StatusCode
			respBody, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				lastErr = err
				statusCode = 0
			} else if resp.StatusCode == http.StatusOK {
				// Success!
				slog.Info("received tagger response from Gemini", "community", in.CommunityKey, "raw_response", string(respBody))
				text, err := extractText(respBody)
				if err != nil {
					return communitytrend.TaggerOutput{}, err
				}

				var out communitytrend.TaggerOutput
				if err := json.Unmarshal([]byte(stripCodeFence(text)), &out); err != nil {
					return communitytrend.TaggerOutput{}, fmt.Errorf("parse tagger json: %w (raw: %s)", err, text)
				}
				return out, nil
			} else {
				lastErr = fmt.Errorf("gemini tagger error (status %d): %s", resp.StatusCode, string(respBody))
			}
		}

		// If it's a non-retryable error (e.g. 400 Bad Request, 401 Unauthorized, 403 Forbidden), don't retry.
		// Retryable: 429 Too Many Requests, 500 Internal Server Error, 502 Bad Gateway, 503 Service Unavailable, 504 Gateway Timeout, or client connection errors (statusCode == 0).
		isRetryable := statusCode == 0 ||
			statusCode == http.StatusTooManyRequests ||
			statusCode == http.StatusInternalServerError ||
			statusCode == http.StatusBadGateway ||
			statusCode == http.StatusServiceUnavailable ||
			statusCode == http.StatusGatewayTimeout

		if !isRetryable || attempt == maxAttempts {
			break
		}

		// Exponential backoff
		delay := baseDelay * time.Duration(1<<(attempt-1))
		slog.Warn("AI call failed, retrying with backoff",
			"community", in.CommunityKey,
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"delay", delay.String(),
			"status_code", statusCode,
			"error", lastErr.Error(),
		)

		select {
		case <-ctx.Done():
			return communitytrend.TaggerOutput{}, ctx.Err()
		case <-time.After(delay):
		}
	}

	return communitytrend.TaggerOutput{}, fmt.Errorf("AI call failed after %d attempts: %w", maxAttempts, lastErr)
}

// extractText reuses the same response shape as the collector client.
func extractText(raw []byte) (string, error) {
	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return "", fmt.Errorf("unmarshal tagger response: %w", err)
	}
	if len(apiResp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in tagger response: %s", string(raw))
	}
	var parts []string
	for _, p := range apiResp.Candidates[0].Content.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	out := strings.Join(parts, "")
	if out == "" {
		return "", fmt.Errorf("empty tagger output: %s", string(raw))
	}
	return out, nil
}

// stripCodeFence removes ```json ... ``` wrappers the model sometimes adds.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// --- request types (JSON output, no search tool) ---

type ctRequest struct {
	Contents         []ctContent         `json:"contents"`
	GenerationConfig *ctGenerationConfig `json:"generationConfig,omitempty"`
}

type ctGenerationConfig struct {
	ResponseMimeType string          `json:"responseMimeType,omitempty"`
	ThinkingConfig   *thinkingConfig `json:"thinkingConfig,omitempty"`
}

type ctContent struct {
	Parts []ctPart `json:"parts"`
}

type ctPart struct {
	Text string `json:"text"`
}
