// Package vertex adapts Google's Vertex AI (express / API-key mode) to the
// collector's text and image generation interfaces.
//
// Express mode is selected by setting Backend=BackendVertexAI together with an
// API key and NO project/location. The genai SDK treats project/location and
// API key as mutually exclusive — passing both returns an error. This is the
// Go equivalent of the Python `genai.Client(vertexai=True, api_key=KEY)`.
//
// All model IDs and the API key are injected by the caller; nothing here is
// hardcoded, so swapping the model is a config-only change.
package vertex

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ota/domain/collector"

	"google.golang.org/genai"
)

// newGenaiClient creates a genai client in Vertex AI express (API-key) mode.
//
// NOTE: do not set Project/Location here — in express mode they are mutually
// exclusive with APIKey and the SDK rejects the config ("project/location and
// API key are mutually exclusive").
func newGenaiClient(ctx context.Context, apiKey string) (*genai.Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("vertex: api key is required")
	}
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("vertex: creating genai client: %w", err)
	}
	return client, nil
}

// TextClient implements collector.AIClient using Vertex AI text generation with
// Google Search grounding.
type TextClient struct {
	client *genai.Client
	model  string
}

// Compile-time check that TextClient satisfies the domain interface.
var _ collector.AIClient = (*TextClient)(nil)

// NewTextClient creates a Vertex text-generation adapter for the given model.
func NewTextClient(ctx context.Context, apiKey, model string) (*TextClient, error) {
	if model == "" {
		return nil, fmt.Errorf("vertex: text model is required")
	}
	client, err := newGenaiClient(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	return &TextClient{client: client, model: model}, nil
}

// SearchAndAnalyze sends the prompt to the model with Google Search grounding,
// returning the generated text plus any grounding source annotations.
func (c *TextClient) SearchAndAnalyze(ctx context.Context, prompt string) (collector.AIResponse, error) {
	cfg := &genai.GenerateContentConfig{
		Tools: []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}},
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingBudget: genai.Ptr[int32](-1), // dynamic: model decides budget per prompt
		},
	}

	result, err := c.client.Models.GenerateContent(ctx, c.model, genai.Text(prompt), cfg)
	if err != nil {
		return collector.AIResponse{}, fmt.Errorf("vertex generate content: %w", err)
	}
	if result == nil || len(result.Candidates) == 0 {
		return collector.AIResponse{}, fmt.Errorf("vertex: no candidates in response")
	}

	candidate := result.Candidates[0]

	var sb strings.Builder
	if candidate.Content != nil {
		for _, p := range candidate.Content.Parts {
			if p.Thought { // skip internal thinking tokens
				continue
			}
			sb.WriteString(p.Text)
		}
	}
	outputText := sb.String()
	if outputText == "" {
		return collector.AIResponse{}, fmt.Errorf("vertex: no output text in response")
	}

	resp := collector.AIResponse{
		OutputText: outputText,
		RawJSON:    marshalRaw(result),
	}
	if candidate.GroundingMetadata != nil {
		for _, chunk := range candidate.GroundingMetadata.GroundingChunks {
			if chunk.Web != nil && chunk.Web.URI != "" {
				resp.Annotations = append(resp.Annotations, collector.AIAnnotation{
					URL:   chunk.Web.URI,
					Title: chunk.Web.Title,
				})
			}
		}
	}
	return resp, nil
}

// marshalRaw serializes the SDK response for checkpoint storage. Best-effort:
// on error it returns an empty string rather than failing the request.
func marshalRaw(result *genai.GenerateContentResponse) string {
	b, err := json.Marshal(result)
	if err != nil {
		return ""
	}
	return string(b)
}
