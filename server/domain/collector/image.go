package collector

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ImageGeneratorClient generates an image from a text prompt.
// Returns (imageBytes, mimeType, error). If generation fails or produces no
// image, imageBytes is nil.
type ImageGeneratorClient interface {
	Generate(ctx context.Context, prompt string) ([]byte, string, error)
}

// ImageGenerator orchestrates image generation and local file storage.
type ImageGenerator struct {
	client  ImageGeneratorClient
	baseDir string // e.g. "data/images"
}

// NewImageGenerator creates an ImageGenerator that saves files under baseDir.
func NewImageGenerator(client ImageGeneratorClient, baseDir string) *ImageGenerator {
	return &ImageGenerator{client: client, baseDir: baseDir}
}

// GenerateForItems generates thumbnail images for the given context items.
// It mutates nothing — returns a map of itemID → relative file path for items
// that succeeded. Failures are logged and skipped (no retry).
func (g *ImageGenerator) GenerateForItems(ctx context.Context, items []ContextItem) map[uuid.UUID]string {
	result := make(map[uuid.UUID]string, len(items))

	for _, item := range items {
		select {
		case <-ctx.Done():
			return result
		default:
		}

		path, err := g.generateOne(ctx, item)
		if err != nil {
			fmt.Printf("image generation failed for item %s (%s): %v\n", item.ID, item.Topic, err)
			continue
		}
		if path != "" {
			result[item.ID] = path
		}
	}

	return result
}

func (g *ImageGenerator) generateOne(ctx context.Context, item ContextItem) (string, error) {
	prompt := buildImagePrompt(item.Topic)

	data, mimeType, err := g.client.Generate(ctx, prompt)
	if err != nil {
		return "", err
	}
	if data == nil {
		return "", nil
	}

	ext := extensionFromMIME(mimeType)
	relPath := g.buildPath(item.ID, ext)
	absPath := filepath.Join(g.baseDir, relPath)

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("creating image directory: %w", err)
	}
	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return "", fmt.Errorf("writing image file: %w", err)
	}

	return relPath, nil
}

// buildPath creates a human-friendly relative path: 2026/03/07/{id}.png
func (g *ImageGenerator) buildPath(itemID uuid.UUID, ext string) string {
	now := time.Now().UTC().Add(9 * time.Hour) // KST
	return filepath.Join(
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
		itemID.String()+ext,
	)
}

// buildImagePrompt combines the common header with a randomly selected style.
func buildImagePrompt(topic string) string {
	header := fmt.Sprintf(string(commonHeader), topic)
	style := imgStylePrompts[rand.IntN(len(imgStylePrompts))]
	return header + string(style)
}

func extensionFromMIME(mime string) string {
	switch {
	case strings.Contains(mime, "jpeg"), strings.Contains(mime, "jpg"):
		return ".jpg"
	case strings.Contains(mime, "webp"):
		return ".webp"
	default:
		return ".png"
	}
}
