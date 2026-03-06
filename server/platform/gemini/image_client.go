package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// ImageClient generates images via the Gemini API using the official Go SDK.
type ImageClient struct {
	client *genai.Client
	model  string
}

// NewImageClient creates a new ImageClient.
// The apiKey is used for authentication; model is the Gemini image model name.
func NewImageClient(ctx context.Context, apiKey, model string) (*ImageClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("creating genai client: %w", err)
	}
	return &ImageClient{client: client, model: model}, nil
}

// Generate sends a text prompt to the Gemini image model and returns the
// generated image bytes (PNG) along with the MIME type.
// Returns (nil, "", nil) if the model produced no image in its response.
func (c *ImageClient) Generate(ctx context.Context, prompt string) ([]byte, string, error) {
	result, err := c.client.Models.GenerateContent(ctx, c.model, genai.Text(prompt), nil)
	if err != nil {
		return nil, "", fmt.Errorf("image generation: %w", err)
	}

	if result == nil || len(result.Candidates) == 0 {
		return nil, "", nil
	}

	for _, part := range result.Candidates[0].Content.Parts {
		if part.InlineData != nil && len(part.InlineData.Data) > 0 {
			return part.InlineData.Data, part.InlineData.MIMEType, nil
		}
	}

	return nil, "", nil
}
