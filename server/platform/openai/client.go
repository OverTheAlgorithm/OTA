package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ota/domain/collector"
)

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
		Model: c.model,
		Tools: []tool{{Type: "web_search"}},
		Input: prompt,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return collector.AIResponse{}, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(jsonBody))
	if err != nil {
		return collector.AIResponse{}, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		return collector.AIResponse{}, fmt.Errorf("openai api error (status %d): %s", resp.StatusCode, string(rawBytes))
	}

	return parseResponse(rawBytes)
}

func parseResponse(raw []byte) (collector.AIResponse, error) {
	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return collector.AIResponse{}, fmt.Errorf("unmarshaling response: %w", err)
	}

	result := collector.AIResponse{
		RawJSON: string(raw),
	}

	for _, item := range apiResp.Output {
		if item.Type != "message" {
			continue
		}
		for _, content := range item.Content {
			if content.Type == "output_text" {
				result.OutputText = content.Text
				for _, ann := range content.Annotations {
					if ann.URLCitation != nil {
						result.Annotations = append(result.Annotations, collector.AIAnnotation{
							URL:   ann.URLCitation.URL,
							Title: ann.URLCitation.Title,
						})
					}
				}
			}
		}
	}

	if result.OutputText == "" {
		return collector.AIResponse{}, fmt.Errorf("no output text in response: %s", string(raw))
	}

	return result, nil
}

type requestBody struct {
	Model string `json:"model"`
	Tools []tool `json:"tools"`
	Input string `json:"input"`
}

type tool struct {
	Type string `json:"type"`
}

type apiResponse struct {
	Output []outputItem `json:"output"`
}

type outputItem struct {
	Type    string         `json:"type"`
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type        string       `json:"type"`
	Text        string       `json:"text"`
	Annotations []annotation `json:"annotations"`
}

type annotation struct {
	URLCitation *urlCitation `json:"url_citation"`
}

type urlCitation struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}
