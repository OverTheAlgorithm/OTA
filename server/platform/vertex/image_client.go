package vertex

import (
	"context"
	"fmt"

	"ota/domain/collector"

	"google.golang.org/genai"
)

// ImageClient implements collector.ImageGeneratorClient using Vertex AI image
// generation. The model must be image-capable, and the request must ask for
// both TEXT and IMAGE modalities — otherwise the model returns no image.
type ImageClient struct {
	client *genai.Client
	model  string
}

// Compile-time check that ImageClient satisfies the domain interface.
var _ collector.ImageGeneratorClient = (*ImageClient)(nil)

// NewImageClient creates a Vertex image-generation adapter for the given model.
func NewImageClient(ctx context.Context, apiKey, model string) (*ImageClient, error) {
	if model == "" {
		return nil, fmt.Errorf("vertex: image model is required")
	}
	client, err := newGenaiClient(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	return &ImageClient{client: client, model: model}, nil
}

// Generate sends a text prompt to the image model and returns the generated
// image bytes plus MIME type. Returns (nil, "", nil) if the model produced no
// image in its response.
func (c *ImageClient) Generate(ctx context.Context, prompt string) ([]byte, string, error) {
	cfg := &genai.GenerateContentConfig{
		// Image models only emit an image when IMAGE is among the requested
		// response modalities; TEXT must accompany it.
		ResponseModalities: []string{"TEXT", "IMAGE"},
	}

	result, err := c.client.Models.GenerateContent(ctx, c.model, genai.Text(prompt), cfg)
	if err != nil {
		return nil, "", fmt.Errorf("vertex image generation: %w", err)
	}
	if result == nil || len(result.Candidates) == 0 || result.Candidates[0].Content == nil {
		return nil, "", nil
	}

	for _, part := range result.Candidates[0].Content.Parts {
		if part.InlineData != nil && len(part.InlineData.Data) > 0 {
			return part.InlineData.Data, part.InlineData.MIMEType, nil
		}
	}
	return nil, "", nil
}
