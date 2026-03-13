package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// URLDecoder decodes redirect URLs in-place across the given string slices.
// Returns the number of URLs successfully decoded.
type URLDecoder func(ctx context.Context, urlSlices ...[]string) int

type Service struct {
	ai             AIClient
	fallbackAI     AIClient // optional: used when primary fails with 5xx after all retries
	repo           Repository
	aggregator     *Aggregator
	trendingRepo   TrendingItemRepository
	brainCatRepo   BrainCategoryRepository
	urlDecoder     URLDecoder
	articleFetcher ArticleFetcher
	imageGen       *ImageGenerator
}

func NewService(
	aiClient AIClient,
	repo Repository,
	aggregator *Aggregator,
	trendingRepo TrendingItemRepository,
	brainCatRepo BrainCategoryRepository,
	urlDecoder URLDecoder,
	articleFetcher ArticleFetcher,
	imageGen *ImageGenerator,
) *Service {
	return &Service{
		ai:             aiClient,
		repo:           repo,
		aggregator:     aggregator,
		trendingRepo:   trendingRepo,
		brainCatRepo:   brainCatRepo,
		urlDecoder:     urlDecoder,
		articleFetcher: articleFetcher,
		imageGen:       imageGen,
	}
}

// WithFallback sets a fallback AI client to use when the primary model returns
// infrastructure errors (HTTP 5xx) after exhausting all retries.
// The fallback itself also gets the same number of retries.
func (s *Service) WithFallback(fallbackAI AIClient) *Service {
	s.fallbackAI = fallbackAI
	return s
}

// CollectFromSources runs the two-phase collection pipeline:
// Stage 0: Collect from structured sources (Google Trends, Google News, etc.)
// Stage 1: Phase 1 AI — cluster, categorize, buzz_score, select sources
// Stage 2: Decode redirect URLs (e.g. Google News) to original article URLs
// Stage 3: Fetch article content (HTTP GET + HTML→text)
// Stage 4: Phase 2 AI — per-topic writing with article content (parallel)
// Stage 5: Validate source URLs — remove invalid ones
// Stage 6: Generate thumbnail images
// Stage 7: Save
func (s *Service) CollectFromSources(ctx context.Context) (CollectionResult, error) {
	run := CollectionRun{
		ID:        uuid.New(),
		StartedAt: time.Now().UTC(),
		Status:    RunStatusRunning,
	}

	if err := s.repo.CreateRun(ctx, run); err != nil {
		return CollectionResult{}, fmt.Errorf("creating collection run: %w", err)
	}

	failRun := func(err error, rawResp *string) (CollectionResult, error) {
		errMsg := err.Error()
		_ = s.repo.CompleteRun(ctx, run.ID, RunStatusFailed, &errMsg, rawResp)
		return CollectionResult{}, err
	}

	// Stage 0: collect from structured sources (no AI involved).
	log.Printf("collection run %s: stage 0 — structured source collection", run.ID)
	data, err := s.aggregator.Collect(ctx)
	if err != nil {
		return failRun(fmt.Errorf("source collection: %w", err), nil)
	}
	log.Printf("collection run %s: stage 0 done — %d items from sources", run.ID, len(data.Items))

	// Persist raw trending data for tracking/analysis.
	if err := s.trendingRepo.SaveTrendingItems(ctx, run.ID, data.Items); err != nil {
		log.Printf("warning: failed to save trending items: %v", err)
	}

	// Load brain categories for AI prompt injection.
	brainCategories, err := s.brainCatRepo.GetAll(ctx)
	if err != nil {
		log.Printf("warning: failed to load brain categories: %v — proceeding without them", err)
		brainCategories = nil
	}

	// Stage 1: Phase 1 AI — clustering + categorization + buzz_score + source selection.
	log.Printf("collection run %s: stage 1 — Phase 1 AI clustering", run.ID)
	phase1Prompt := BuildClusterPrompt(data.FormattedText, brainCategories)
	phase1Resp, err := s.callAIWithRetry(ctx, phase1Prompt)
	if err != nil {
		return failRun(fmt.Errorf("Phase 1 AI clustering: %w", err), nil)
	}

	topics, err := parsePhase1Response(phase1Resp.OutputText)
	if err != nil {
		rawResp := phase1Resp.RawJSON
		return failRun(fmt.Errorf("parsing Phase 1 response: %w", err), &rawResp)
	}
	log.Printf("collection run %s: stage 1 done — %d topics clustered", run.ID, len(topics))

	// Stage 2: Decode redirect URLs to original article URLs.
	topics = s.decodePhase1URLs(ctx, run.ID, topics)

	// Stage 3: Fetch article content for each topic.
	log.Printf("collection run %s: stage 3 — fetching article content", run.ID)
	articleMap := s.fetchArticlesForTopics(ctx, topics)
	log.Printf("collection run %s: stage 3 done — fetched articles for %d topics", run.ID, len(articleMap))

	// Stage 4: Phase 2 AI — per-topic detail writing (parallel with semaphore).
	log.Printf("collection run %s: stage 4 — Phase 2 AI detail writing", run.ID)
	items, phase2RawResponses := s.runPhase2(ctx, run.ID, topics, articleMap, brainCategories)
	if len(items) == 0 {
		rawResp := phase1Resp.RawJSON
		return failRun(fmt.Errorf("Phase 2 AI: all topics failed"), &rawResp)
	}
	log.Printf("collection run %s: stage 4 done — %d/%d topics written", run.ID, len(items), len(topics))

	// Stage 5: Validate source URLs — remove invalid ones.
	items = s.validateAndRemoveSources(ctx, run.ID, items)

	// Stage 6: Generate thumbnail images (optional, best-effort).
	items = s.generateImages(ctx, run.ID, items)

	// Build combined raw response for debugging (before save so it's available on failure).
	combinedRaw := buildCombinedRawResponse(phase1Resp.RawJSON, phase2RawResponses)

	// Stage 7: Save.
	if err := s.repo.SaveContextItems(ctx, items); err != nil {
		return failRun(fmt.Errorf("saving context items: %w", err), &combinedRaw)
	}

	now := time.Now().UTC()
	run.CompletedAt = &now
	run.Status = RunStatusSuccess
	run.RawResponse = &combinedRaw

	if err := s.repo.CompleteRun(ctx, run.ID, RunStatusSuccess, nil, &combinedRaw); err != nil {
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

// --- Phase 1 parsing ---

type phase1Payload struct {
	Topics []Phase1Topic `json:"topics"`
}

func parsePhase1Response(outputText string) ([]Phase1Topic, error) {
	cleanJSON := stripMarkdownCodeFence(outputText)

	var payload phase1Payload
	if err := json.Unmarshal([]byte(cleanJSON), &payload); err != nil {
		return nil, fmt.Errorf("invalid json from Phase 1 AI: %w", err)
	}

	if len(payload.Topics) == 0 {
		return nil, fmt.Errorf("Phase 1 AI returned empty topics")
	}

	// Filter out invalid topics.
	valid := make([]Phase1Topic, 0, len(payload.Topics))
	for _, t := range payload.Topics {
		if t.TopicHint == "" || t.Category == "" || len(t.Sources) == 0 {
			continue
		}
		valid = append(valid, t)
	}

	if len(valid) == 0 {
		return nil, fmt.Errorf("no valid topics after filtering")
	}
	return valid, nil
}

// rankForTopic returns the rank of a topic within its category peers (1 = highest buzz_score).
func rankForTopic(topic Phase1Topic, allTopics []Phase1Topic) int {
	rank := 1
	for _, t := range allTopics {
		if t.Category == topic.Category && t.BuzzScore > topic.BuzzScore {
			rank++
		}
	}
	return rank
}

// --- Phase 1 URL decoding ---

func (s *Service) decodePhase1URLs(ctx context.Context, runID uuid.UUID, topics []Phase1Topic) []Phase1Topic {
	log.Printf("collection run %s: stage 2 — decoding redirect URLs", runID)

	result := make([]Phase1Topic, len(topics))
	var slices [][]string
	for i, t := range topics {
		result[i] = t
		srcCopy := make([]string, len(t.Sources))
		copy(srcCopy, t.Sources)
		result[i].Sources = srcCopy
		slices = append(slices, result[i].Sources)
	}

	decoded := s.urlDecoder(ctx, slices...)
	log.Printf("collection run %s: stage 2 done — decoded %d URLs across %d topics", runID, decoded, len(topics))
	return result
}

// --- Article fetching ---

func (s *Service) fetchArticlesForTopics(ctx context.Context, topics []Phase1Topic) map[int][]FetchedArticle {
	result := make(map[int][]FetchedArticle, len(topics))
	for i, t := range topics {
		articles := s.articleFetcher(ctx, t.Sources)
		result[i] = articles
	}
	return result
}

// --- Phase 2 parallel execution ---

const phase2Concurrency = 5

func (s *Service) runPhase2(
	ctx context.Context,
	runID uuid.UUID,
	topics []Phase1Topic,
	articleMap map[int][]FetchedArticle,
	brainCategories []BrainCategory,
) ([]ContextItem, []string) {
	type phase2Output struct {
		index int
		item  ContextItem
		raw   string
		err   error
	}

	results := make(chan phase2Output, len(topics))
	sem := make(chan struct{}, phase2Concurrency)
	var wg sync.WaitGroup

	for i, topic := range topics {
		wg.Add(1)
		go func(idx int, t Phase1Topic) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			articles := articleMap[idx]

			// Drop topic if no article content was fetched — writing without sources produces hallucinations.
			hasContent := false
			for _, a := range articles {
				if a.Err == nil && a.Body != "" {
					hasContent = true
					break
				}
			}
			if !hasContent {
				log.Printf("collection run %s: dropping topic %q — no article content available", runID, t.TopicHint)
				results <- phase2Output{index: idx, err: fmt.Errorf("no article content for topic %q", t.TopicHint)}
				return
			}

			prompt := BuildDetailPrompt(t, articles, brainCategories)

			resp, err := s.callAIWithRetry(ctx, prompt)
			if err != nil {
				log.Printf("collection run %s: Phase 2 failed for topic %q: %v", runID, t.TopicHint, err)
				results <- phase2Output{index: idx, err: err}
				return
			}

			p2Result, err := parsePhase2Response(resp.OutputText)
			if err != nil {
				log.Printf("collection run %s: Phase 2 parse failed for topic %q: %v", runID, t.TopicHint, err)
				results <- phase2Output{index: idx, err: err, raw: resp.RawJSON}
				return
			}

			item := ContextItem{
				ID:              uuid.New(),
				CollectionRunID: runID,
				Category:        t.Category,
				BrainCategory:   t.BrainCategory,
				Rank:            rankForTopic(t, topics),
				Topic:           p2Result.Topic,
				Summary:         p2Result.Summary,
				Detail:          p2Result.Detail,
				Details:         p2Result.Details,
				BuzzScore:       t.BuzzScore,
				Sources:         t.Sources,
			}

			results <- phase2Output{index: idx, item: item, raw: resp.RawJSON}
		}(i, topic)
	}

	wg.Wait()
	close(results)

	var items []ContextItem
	rawResponses := make([]string, 0, len(topics))
	for out := range results {
		if out.err != nil {
			if out.raw != "" {
				rawResponses = append(rawResponses, out.raw)
			}
			continue
		}
		items = append(items, out.item)
		rawResponses = append(rawResponses, out.raw)
	}

	return items, rawResponses
}

func parsePhase2Response(outputText string) (Phase2Result, error) {
	cleanJSON := stripMarkdownCodeFence(outputText)

	var result Phase2Result
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return Phase2Result{}, fmt.Errorf("invalid json from Phase 2 AI: %w", err)
	}

	if result.Topic == "" || result.Summary == "" {
		return Phase2Result{}, fmt.Errorf("Phase 2 AI returned empty topic or summary")
	}

	return result, nil
}

// --- Retry logic ---

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

// --- Utilities ---

// stripMarkdownCodeFence removes markdown code fence markers from text.
// Handles formats like: ```json\n{...}\n``` or ```\n{...}\n```
func stripMarkdownCodeFence(text string) string {
	text = strings.TrimSpace(text)

	if !strings.HasPrefix(text, "```") {
		return text
	}

	firstNewline := strings.Index(text, "\n")
	if firstNewline == -1 {
		return ""
	}

	text = text[firstNewline+1:]

	closingIndex := strings.LastIndex(text, "```")
	if closingIndex >= 0 {
		text = text[:closingIndex]
	}

	return strings.TrimSpace(text)
}

// buildCombinedRawResponse merges Phase 1 and Phase 2 raw JSON for debugging.
func buildCombinedRawResponse(phase1Raw string, phase2Raws []string) string {
	combined := map[string]any{
		"phase1": json.RawMessage(phase1Raw),
	}
	if len(phase2Raws) > 0 {
		rawMessages := make([]json.RawMessage, len(phase2Raws))
		for i, r := range phase2Raws {
			rawMessages[i] = json.RawMessage(r)
		}
		combined["phase2"] = rawMessages
	}
	b, err := json.Marshal(combined)
	if err != nil {
		return phase1Raw
	}
	return string(b)
}

// --- Stage helpers (unchanged from original) ---

// validateAndRemoveSources checks all source URLs and removes invalid ones.
// Returns a new slice — the input is not mutated.
func (s *Service) validateAndRemoveSources(ctx context.Context, runID uuid.UUID, items []ContextItem) []ContextItem {
	log.Printf("collection run %s: stage 5 — validating source URLs", runID)

	validator := NewSourceValidator()
	invalid := validator.ValidateSources(ctx, items)
	if len(invalid) == 0 {
		log.Printf("collection run %s: stage 5 done — all source URLs valid", runID)
		return items
	}

	log.Printf("collection run %s: stage 5 done — removed %d invalid URL(s)", runID, len(invalid))
	return removeInvalidSources(items, invalid)
}

// generateImages creates thumbnail images for each item using the image generator.
// Returns a new slice with ImagePath populated for items that succeeded.
func (s *Service) generateImages(ctx context.Context, runID uuid.UUID, items []ContextItem) []ContextItem {
	log.Printf("collection run %s: stage 6 — generating thumbnail images", runID)

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

	log.Printf("collection run %s: stage 6 done — %d/%d images generated", runID, generated, len(items))
	return result
}

// removeInvalidSources strips all invalid URLs from items. Returns a new slice.
func removeInvalidSources(items []ContextItem, invalid []InvalidSource) []ContextItem {
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
