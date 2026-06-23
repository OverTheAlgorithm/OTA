package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return communitytrend.TaggerOutput{}, fmt.Errorf("create tagger request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return communitytrend.TaggerOutput{}, fmt.Errorf("send tagger request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return communitytrend.TaggerOutput{}, fmt.Errorf("read tagger response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return communitytrend.TaggerOutput{}, fmt.Errorf("gemini tagger error (status %d): %s", resp.StatusCode, string(raw))
	}

	text, err := extractText(raw)
	if err != nil {
		return communitytrend.TaggerOutput{}, err
	}

	var out communitytrend.TaggerOutput
	if err := json.Unmarshal([]byte(stripCodeFence(text)), &out); err != nil {
		return communitytrend.TaggerOutput{}, fmt.Errorf("parse tagger json: %w (raw: %s)", err, text)
	}
	return out, nil
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
