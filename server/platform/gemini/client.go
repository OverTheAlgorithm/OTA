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
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) SearchAndAnalyze(ctx context.Context, prompt string) (collector.AIResponse, error) {
	body := requestBody{
		Contents: []content{
			{Parts: []part{{Text: prompt}}},
		},
		Tools: []tool{{GoogleSearch: &googleSearch{}}},
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
	Contents []content `json:"contents"`
	Tools    []tool    `json:"tools"`
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
	Content          responseContent   `json:"content"`
	GroundingMetadata *groundingMetadata `json:"groundingMetadata"`
}

type responseContent struct {
	Parts []part `json:"parts"`
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
