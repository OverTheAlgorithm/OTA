package collector

import (
	"context"
	"fmt"
	"log/slog"
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

// defaultImageBackoff is the initial wait before retrying a failed generation.
const defaultImageBackoff = 5 * time.Second

// ImageGenerator orchestrates image generation and local file storage.
type ImageGenerator struct {
	client  ImageGeneratorClient
	baseDir string // e.g. "data/images"

	throttle    time.Duration // delay inserted before each generation after the first; 0 = none
	maxAttempts int           // total attempts per item (1 = no retry)
	backoff     time.Duration // initial backoff between retries (doubles each attempt)
}

// NewImageGenerator creates an ImageGenerator that saves files under baseDir.
// By default there is no throttle and no retry (maxAttempts=1), preserving the
// original best-effort behavior. Use WithThrottle / WithRetry to tune for rate
// limits (e.g. the Vertex express key's per-minute image quota).
func NewImageGenerator(client ImageGeneratorClient, baseDir string) *ImageGenerator {
	return &ImageGenerator{
		client:      client,
		baseDir:     baseDir,
		maxAttempts: 1,
		backoff:     defaultImageBackoff,
	}
}

// WithThrottle sets a delay applied before each generation after the first one,
// spacing out API calls to stay under per-minute quotas. d <= 0 disables it.
func (g *ImageGenerator) WithThrottle(d time.Duration) *ImageGenerator {
	g.throttle = d
	return g
}

// WithRetry sets how many times a single item is attempted on retryable errors
// (e.g. HTTP 429 / RESOURCE_EXHAUSTED, 5xx). maxAttempts < 1 is treated as 1.
// initialBackoff <= 0 falls back to the default.
func (g *ImageGenerator) WithRetry(maxAttempts int, initialBackoff time.Duration) *ImageGenerator {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	g.maxAttempts = maxAttempts
	if initialBackoff > 0 {
		g.backoff = initialBackoff
	}
	return g
}

// GenerateForItems generates thumbnail images for the given context items.
// It mutates nothing — returns a map of itemID → relative file path for items
// that succeeded. Failures are logged and skipped (no retry).
func (g *ImageGenerator) GenerateForItems(ctx context.Context, items []ContextItem) map[uuid.UUID]string {
	result := make(map[uuid.UUID]string, len(items))

	for i, item := range items {
		select {
		case <-ctx.Done():
			slog.Warn("image generation cancelled by context", "completed", len(result), "total", len(items))
			return result
		default:
		}

		// Space out calls (after the first) to stay under per-minute quotas.
		if i > 0 && g.throttle > 0 {
			select {
			case <-ctx.Done():
				slog.Warn("image generation cancelled by context", "completed", len(result), "total", len(items))
				return result
			case <-time.After(g.throttle):
			}
		}

		path, err := g.generateOne(ctx, item)
		if err != nil {
			slog.Warn("image generation failed", "item_id", item.ID, "category", item.Category, "topic", item.Topic, "error", err)
			continue
		}
		if path == "" {
			slog.Warn("image generation returned nil data", "item_id", item.ID, "category", item.Category, "topic", item.Topic)
			continue
		}
		result[item.ID] = path
	}

	return result
}

func (g *ImageGenerator) generateOne(ctx context.Context, item ContextItem) (string, error) {
	prompt := buildImagePrompt(item.Topic)

	data, mimeType, err := g.generateWithRetry(ctx, prompt, item)
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

// generateWithRetry calls the client up to maxAttempts times, retrying only on
// retryable errors (429 / RESOURCE_EXHAUSTED, 5xx, network) with exponential
// backoff. Non-retryable errors return immediately.
func (g *ImageGenerator) generateWithRetry(ctx context.Context, prompt string, item ContextItem) ([]byte, string, error) {
	backoff := g.backoff
	var lastErr error

	for attempt := 1; attempt <= g.maxAttempts; attempt++ {
		data, mimeType, err := g.client.Generate(ctx, prompt)
		if err == nil {
			return data, mimeType, nil
		}
		lastErr = err

		if !ClassifyError(err).IsRetryable() || attempt == g.maxAttempts {
			return nil, "", err
		}

		slog.Warn("image generation retry",
			"item_id", item.ID, "attempt", attempt, "max", g.maxAttempts, "backoff", backoff, "error", err)

		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}

	return nil, "", lastErr
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
