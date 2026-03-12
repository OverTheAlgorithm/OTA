package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

// URLDecoder decodes redirect URLs in-place across the given string slices.
// Returns the number of URLs successfully decoded.
type URLDecoder func(ctx context.Context, urlSlices ...[]string) int

type Service struct {
	ai           AIClient
	fallbackAI   AIClient // optional: used when primary fails with 5xx after all retries
	repo         Repository
	aggregator   *Aggregator
	trendingRepo TrendingItemRepository
	brainCatRepo BrainCategoryRepository
	urlDecoder   URLDecoder
	imageGen     *ImageGenerator
}

func NewService(
	aiClient AIClient,
	repo Repository,
	aggregator *Aggregator,
	trendingRepo TrendingItemRepository,
	brainCatRepo BrainCategoryRepository,
	urlDecoder URLDecoder,
	imageGen *ImageGenerator,
) *Service {
	return &Service{
		ai:           aiClient,
		repo:         repo,
		aggregator:   aggregator,
		trendingRepo: trendingRepo,
		brainCatRepo: brainCatRepo,
		urlDecoder:   urlDecoder,
		imageGen:     imageGen,
	}
}

// WithFallback sets a fallback AI client to use when the primary model returns
// infrastructure errors (HTTP 5xx) after exhausting all retries.
// The fallback itself also gets the same number of retries.
func (s *Service) WithFallback(fallbackAI AIClient) *Service {
	s.fallbackAI = fallbackAI
	return s
}

// CollectFromSources runs the structured source collection pipeline:
// Stage 0: Collect from structured sources (Google Trends, Google News, etc.)
// Stage 1: AI clusters, ranks, and summarizes the collected data
// Stage 2: Decode redirect URLs (e.g. Google News) to original article URLs
// Stage 3: Validate source URLs — remove invalid ones
func (s *Service) CollectFromSources(ctx context.Context) (CollectionResult, error) {
	run := CollectionRun{
		ID:        uuid.New(),
		StartedAt: time.Now().UTC(),
		Status:    RunStatusRunning,
	}

	if err := s.repo.CreateRun(ctx, run); err != nil {
		return CollectionResult{}, fmt.Errorf("creating collection run: %w", err)
	}

	// Stage 0: collect from structured sources (no AI involved).
	log.Printf("collection run %s: stage 0 — structured source collection", run.ID)
	data, err := s.aggregator.Collect(ctx)
	if err != nil {
		errMsg := err.Error()
		_ = s.repo.CompleteRun(ctx, run.ID, RunStatusFailed, &errMsg, nil)
		return CollectionResult{}, fmt.Errorf("source collection: %w", err)
	}
	log.Printf("collection run %s: stage 0 done — %d items from sources", run.ID, len(data.Items))

	// Persist raw trending data for tracking/analysis.
	if err := s.trendingRepo.SaveTrendingItems(ctx, run.ID, data.Items); err != nil {
		log.Printf("warning: failed to save trending items: %v", err)
		// Non-fatal — continue pipeline
	}

	// Load brain categories for AI prompt injection.
	brainCategories, err := s.brainCatRepo.GetAll(ctx)
	if err != nil {
		log.Printf("warning: failed to load brain categories: %v — proceeding without them", err)
		brainCategories = nil
	}

	// Stage 1: AI clustering + summarization (data-grounded, no hallucination).
	log.Printf("collection run %s: stage 1 — AI clustering and summarization", run.ID)
	prompt := BuildSourceBasedPrompt(data.FormattedText, brainCategories)
	aiResp, err := s.callAIWithRetry(ctx, prompt)
	if err != nil {
		errMsg := err.Error()
		_ = s.repo.CompleteRun(ctx, run.ID, RunStatusFailed, &errMsg, nil)
		return CollectionResult{}, fmt.Errorf("AI clustering: %w", err)
	}

	items, err := parseContextItems(aiResp.OutputText, run.ID)
	if err != nil {
		errMsg := err.Error()
		rawResp := aiResp.RawJSON
		_ = s.repo.CompleteRun(ctx, run.ID, RunStatusFailed, &errMsg, &rawResp)
		return CollectionResult{}, fmt.Errorf("parsing AI response: %w", err)
	}

	// Stage 2: decode redirect URLs to original article URLs.
	items = s.decodeSourceURLs(ctx, run.ID, items)

	// Stage 3: validate source URLs — remove invalid ones.
	items = s.validateAndRemoveSources(ctx, run.ID, items)

	// Stage 4: generate thumbnail images (optional, best-effort).
	items = s.generateImages(ctx, run.ID, items)

	if err := s.repo.SaveContextItems(ctx, items); err != nil {
		errMsg := err.Error()
		rawResp := aiResp.RawJSON
		_ = s.repo.CompleteRun(ctx, run.ID, RunStatusFailed, &errMsg, &rawResp)
		return CollectionResult{}, fmt.Errorf("saving context items: %w", err)
	}

	now := time.Now().UTC()
	rawResp := aiResp.RawJSON
	run.CompletedAt = &now
	run.Status = RunStatusSuccess
	run.RawResponse = &rawResp

	if err := s.repo.CompleteRun(ctx, run.ID, RunStatusSuccess, nil, &rawResp); err != nil {
		return CollectionResult{}, fmt.Errorf("completing run: %w", err)
	}

	log.Printf("collection run %s: complete — %d items from %d source items", run.ID, len(items), len(data.Items))
	return CollectionResult{Run: run, Items: items}, nil
}

// CollectFromSourcesIfNeeded checks if collection already ran today and skips if so.
func (s *Service) CollectFromSourcesIfNeeded(ctx context.Context) (*CollectionResult, error) {
	canRun, err := s.repo.CanRunToday(ctx)
	if err != nil {
		return nil, fmt.Errorf("checking run status: %w", err)
	}

	if !canRun {
		return nil, nil
	}

	result, err := s.CollectFromSources(ctx)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

const (
	maxRetries     = 3
	initialBackoff = 1 * time.Second
)

// callAIWithRetry calls the primary AI client with retry logic.
// If all retries fail with an infrastructure error (HTTP 5xx), and a fallback
// client is configured, it retries the same number of times with the fallback.
func (s *Service) callAIWithRetry(ctx context.Context, prompt string) (AIResponse, error) {
	resp, err := s.retryClient(ctx, s.ai, prompt, "primary")
	if err == nil {
		return resp, nil
	}

	aiErr := ClassifyError(err)
	if aiErr.Type == ErrorTypeInfrastructure && s.fallbackAI != nil {
		log.Printf("primary AI exhausted retries with infrastructure error, switching to fallback model")
		return s.retryClient(ctx, s.fallbackAI, prompt, "fallback")
	}

	return AIResponse{}, err
}

// retryClient runs up to maxRetries attempts against the given client using
// exponential backoff. It stops early on non-retryable (format) errors.
func (s *Service) retryClient(ctx context.Context, client AIClient, prompt string, label string) (AIResponse, error) {
	var lastErr error
	backoff := initialBackoff

	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := client.SearchAndAnalyze(ctx, prompt)
		if err == nil {
			return resp, nil
		}

		aiErr := ClassifyError(err)
		lastErr = aiErr

		// Don't retry format errors — retrying won't fix a bad response shape
		if !aiErr.IsRetryable() {
			log.Printf("%s AI error (non-retryable, attempt %d/%d): %v", label, attempt, maxRetries, aiErr)
			return AIResponse{}, aiErr
		}

		if attempt == maxRetries {
			log.Printf("%s AI error (final attempt %d/%d): %v", label, attempt, maxRetries, aiErr)
			break
		}

		log.Printf("%s AI error (attempt %d/%d, retrying in %v): %v", label, attempt, maxRetries, backoff, aiErr)

		select {
		case <-ctx.Done():
			return AIResponse{}, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(backoff):
			backoff *= 2
		}
	}

	return AIResponse{}, fmt.Errorf("%s AI failed after %d attempts: %w", label, maxRetries, lastErr)
}

type aiResponsePayload struct {
	Items []aiContextItem `json:"items"`
}

type aiContextItem struct {
	Category      string       `json:"category"`
	BrainCategory string       `json:"brain_category"`
	Rank          int          `json:"rank"`
	Topic         string       `json:"topic"`
	Summary       string       `json:"summary"`
	Detail        string       `json:"detail"`
	Details       []DetailItem `json:"details"`
	BuzzScore     int          `json:"buzz_score"`
	Sources       []string     `json:"sources"`
}

// stripMarkdownCodeFence removes markdown code fence markers from text
// Handles formats like: ```json\n{...}\n``` or ```\n{...}\n```
func stripMarkdownCodeFence(text string) string {
	text = strings.TrimSpace(text)

	// Check if starts with ```
	if !strings.HasPrefix(text, "```") {
		return text // No code fence, return as-is
	}

	// Find the first newline (end of opening fence line)
	firstNewline := strings.Index(text, "\n")
	if firstNewline == -1 {
		// No newline, just ``` - return empty
		return ""
	}

	// Skip the opening fence line (```json or ```)
	text = text[firstNewline+1:]

	// Find and remove the closing ``` (anywhere in the remaining text)
	closingIndex := strings.LastIndex(text, "```")
	if closingIndex >= 0 {
		text = text[:closingIndex]
	}

	return strings.TrimSpace(text)
}

func parseContextItems(outputText string, runID uuid.UUID) ([]ContextItem, error) {
	// Strip markdown code fences if present (e.g., ```json ... ```)
	cleanJSON := stripMarkdownCodeFence(outputText)

	var payload aiResponsePayload
	if err := json.Unmarshal([]byte(cleanJSON), &payload); err != nil {
		return nil, fmt.Errorf("invalid json from ai: %w", err)
	}

	if len(payload.Items) == 0 {
		return nil, fmt.Errorf("ai returned empty items")
	}

	items := make([]ContextItem, 0, len(payload.Items))
	for _, raw := range payload.Items {
		if raw.Topic == "" || raw.Summary == "" || raw.Category == "" {
			continue
		}
		items = append(items, ContextItem{
			ID:              uuid.New(),
			CollectionRunID: runID,
			Category:        raw.Category,
			BrainCategory:   raw.BrainCategory,
			Rank:            raw.Rank,
			Topic:           raw.Topic,
			Summary:         raw.Summary,
			Detail:          raw.Detail,
			Details:         raw.Details,
			BuzzScore:       raw.BuzzScore,
			Sources:         raw.Sources,
		})
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no valid context items after filtering")
	}

	return items, nil
}

// decodeSourceURLs resolves redirect URLs (e.g. Google News) in ContextItem.Sources
// to their original article URLs. Only decodes the URLs selected by the AI, not all collected URLs.
func (s *Service) decodeSourceURLs(ctx context.Context, runID uuid.UUID, items []ContextItem) []ContextItem {
	log.Printf("collection run %s: stage 2 — decoding redirect URLs", runID)

	result := make([]ContextItem, len(items))
	var slices [][]string
	for i, item := range items {
		result[i] = item
		srcCopy := make([]string, len(item.Sources))
		copy(srcCopy, item.Sources)
		result[i].Sources = srcCopy
		slices = append(slices, result[i].Sources)
	}

	decoded := s.urlDecoder(ctx, slices...)
	log.Printf("collection run %s: stage 2 done — decoded %d URLs across %d items", runID, decoded, len(items))
	return result
}

// validateAndRemoveSources checks all source URLs and removes invalid ones.
// Returns a new slice — the input is not mutated.
func (s *Service) validateAndRemoveSources(ctx context.Context, runID uuid.UUID, items []ContextItem) []ContextItem {
	log.Printf("collection run %s: stage 3 — validating source URLs", runID)

	validator := NewSourceValidator()
	invalid := validator.ValidateSources(ctx, items)
	if len(invalid) == 0 {
		log.Printf("collection run %s: stage 3 done — all source URLs valid", runID)
		return items
	}

	log.Printf("collection run %s: stage 3 done — removed %d invalid URL(s)", runID, len(invalid))
	return removeInvalidSources(items, invalid)
}

// generateImages creates thumbnail images for each item using the image generator.
// Returns a new slice with ImagePath populated for items that succeeded.
func (s *Service) generateImages(ctx context.Context, runID uuid.UUID, items []ContextItem) []ContextItem {
	log.Printf("collection run %s: stage 4 — generating thumbnail images", runID)

	pathMap := s.imageGen.GenerateForItems(ctx, items)

	result := make([]ContextItem, len(items))
	copy(result, items)
	generated := 0
	for i, item := range result {
		if p, ok := pathMap[item.ID]; ok {
			result[i].ImagePath = &p
			generated++
		}
	}

	log.Printf("collection run %s: stage 4 done — %d/%d images generated", runID, generated, len(items))
	return result
}

// removeInvalidSources strips all invalid URLs from items. Returns a new slice.
func removeInvalidSources(items []ContextItem, invalid []InvalidSource) []ContextItem {
	// Build set of URLs to remove, keyed by (itemIndex, url).
	type key struct {
		idx int
		url string
	}
	removeSet := make(map[key]bool)
	for _, inv := range invalid {
		removeSet[key{inv.ItemIndex, inv.URL}] = true
	}

	result := make([]ContextItem, len(items))
	for i, item := range items {
		result[i] = item
		var kept []string
		for _, src := range item.Sources {
			if !removeSet[key{i, src}] {
				kept = append(kept, src)
			}
		}
		result[i].Sources = kept
	}
	return result
}
