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

	"ota/domain/collector"
)

const baseURL = "https://generativelanguage.googleapis.com/v1beta/models/"

// Available model names (as of Feb 2026):
// - gemini-3.1-pro-preview (latest, released 2026-02-19) ← current default
// - gemini-3-flash-preview (flash-tier Gemini 3, released 2025-12-17)
// - gemini-3-pro-preview   (Gemini 3 Pro, released 2025-11-18)
// - gemini-2.5-flash / gemini-2.5-flash-lite (previous gen)
// Note: Using v1beta endpoint for Google Search grounding support
// Note: thinkingBudget -1 = dynamic (model decides based on complexity)

type Client struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewClient(apiKey string, model string) *Client {
	return &Client{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			// Pro + thinking can take significantly longer than flash models
			Timeout: 300 * time.Second,
		},
	}
}

func (c *Client) SearchAndAnalyze(ctx context.Context, prompt string) (collector.AIResponse, error) {
	body := requestBody{
		Contents: []content{
			{Parts: []part{{Text: prompt}}},
		},
		Tools: []tool{{GoogleSearch: &googleSearch{}}},
		GenerationConfig: &generationConfig{
			ThinkingConfig: &thinkingConfig{
				ThinkingBudget: -1, // dynamic: model decides token budget per prompt
			},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return collector.AIResponse{}, fmt.Errorf("marshaling request: %w", err)
	}

	url := baseURL + c.model + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return collector.AIResponse{}, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return collector.AIResponse{}, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	rawBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return collector.AIResponse{}, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return collector.AIResponse{}, fmt.Errorf("gemini api error (status %d): %s", resp.StatusCode, string(rawBytes))
	}

	return parseResponse(rawBytes)
}

func parseResponse(raw []byte) (collector.AIResponse, error) {
	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return collector.AIResponse{}, fmt.Errorf("unmarshaling response: %w", err)
	}

	if len(apiResp.Candidates) == 0 {
		return collector.AIResponse{}, fmt.Errorf("no candidates in response: %s", string(raw))
	}

	candidate := apiResp.Candidates[0]

	var textParts []string
	for _, p := range candidate.Content.Parts {
		// thinking parts have empty Text but non-empty ThinkingText; skip them
		if p.Text != "" {
			textParts = append(textParts, p.Text)
		}
	}

	outputText := strings.Join(textParts, "")
	if outputText == "" {
		return collector.AIResponse{}, fmt.Errorf("no output text in response: %s", string(raw))
	}

	result := collector.AIResponse{
		OutputText: outputText,
		RawJSON:    string(raw),
	}

	if candidate.GroundingMetadata != nil {
		for _, sr := range candidate.GroundingMetadata.SearchResults {
			if sr.URL != "" {
				result.Annotations = append(result.Annotations, collector.AIAnnotation{
					URL:   sr.URL,
					Title: sr.Title,
				})
			}
		}
		for _, chunk := range candidate.GroundingMetadata.GroundingChunks {
			if chunk.Web != nil && chunk.Web.URI != "" {
				result.Annotations = append(result.Annotations, collector.AIAnnotation{
					URL:   chunk.Web.URI,
					Title: chunk.Web.Title,
				})
			}
		}
	}

	return result, nil
}

// --- request types ---

type requestBody struct {
	Contents         []content         `json:"contents"`
	Tools            []tool            `json:"tools"`
	GenerationConfig *generationConfig `json:"generationConfig,omitempty"`
}

type generationConfig struct {
	ThinkingConfig *thinkingConfig `json:"thinkingConfig,omitempty"`
}

type thinkingConfig struct {
	// -1 = dynamic (model decides), 0 = disabled, >0 = fixed token budget
	ThinkingBudget int `json:"thinkingBudget"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text string `json:"text"`
}

type tool struct {
	GoogleSearch *googleSearch `json:"google_search,omitempty"`
}

type googleSearch struct{}

// --- response types ---

type apiResponse struct {
	Candidates []candidate `json:"candidates"`
}

type candidate struct {
	Content           responseContent    `json:"content"`
	GroundingMetadata *groundingMetadata `json:"groundingMetadata"`
}

type responseContent struct {
	Parts []responsePart `json:"parts"`
}

// responsePart separates visible text from internal thinking tokens.
// ThinkingText is populated when the model uses thinking mode; we ignore it.
type responsePart struct {
	Text         string `json:"text"`
	ThinkingText string `json:"thought"` // Gemini thinking token field
}

type groundingMetadata struct {
	SearchResults   []searchResult   `json:"searchResult"`
	GroundingChunks []groundingChunk `json:"groundingChunks"`
}

type searchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type groundingChunk struct {
	Web *webChunk `json:"web"`
}

type webChunk struct {
	URI   string `json:"uri"`
	Title string `json:"title"`
}
