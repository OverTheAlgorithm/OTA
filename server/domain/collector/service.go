package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"ota/domain/poll"
	"ota/domain/quiz"
)

// URLDecoder decodes redirect URLs in-place across the given string slices.
// Returns the number of URLs successfully decoded.
type URLDecoder func(ctx context.Context, urlSlices ...[]string) int

// QuizSaver is a minimal interface for saving quiz data from the pipeline.
// It only needs SaveQuizBatch — the full quiz.Repository is not required here.
type QuizSaver interface {
	SaveQuizBatch(ctx context.Context, quizzes []quiz.Quiz) error
}

// PollSaver is a minimal interface for saving polls produced inline by Phase 2.
type PollSaver interface {
	SavePollBatch(ctx context.Context, polls []poll.Poll) error
}

type Service struct {
	ai             AIClient
	fallbackAI     AIClient // optional: used when primary fails with 5xx after all retries
	repo           Repository
	aggregator     *Aggregator
	trendingRepo   TrendingItemRepository
	brainCatRepo   BrainCategoryRepository
	catRepo        CategoryRepository // optional: loads categories from DB for AI prompts
	quizRepo       QuizSaver          // optional: saves quizzes from collection pipeline
	pollRepo       PollSaver          // optional: saves polls from Phase 2 output
	checkpointRepo CheckpointRepository // optional: enables checkpoint/resume
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

// WithCategoryRepo sets the category repository for loading categories from DB.
func (s *Service) WithCategoryRepo(catRepo CategoryRepository) *Service {
	s.catRepo = catRepo
	return s
}

// WithQuizRepo sets the quiz repository used to persist quizzes in Stage 7.
// If not set, Stage 7 is skipped silently.
func (s *Service) WithQuizRepo(repo QuizSaver) *Service {
	s.quizRepo = repo
	return s
}

// WithPollRepo sets the poll repository used to persist polls emitted by Phase 2.
// If not set, inline poll payloads from the AI are ignored.
func (s *Service) WithPollRepo(repo PollSaver) *Service {
	s.pollRepo = repo
	return s
}

// WithCheckpointRepo sets the checkpoint repository for pipeline resume capability.
// If not set, checkpoint saves are skipped and ResumeOrCollect delegates to CollectFromSources.
func (s *Service) WithCheckpointRepo(repo CheckpointRepository) *Service {
	s.checkpointRepo = repo
	return s
}

// CollectFromSources runs the two-phase collection pipeline:
// Stage 0: Collect from structured sources (Google Trends, Google News, etc.)
// Stage 1: Phase 1 AI — cluster, categorize, buzz_score, select sources
// Stage 2: Decode redirect URLs (e.g. Google News) to original article URLs
// Stage 3: Fetch articles + validate sources (blocked hosts, HTTP errors → drop)
// Stage 4: Phase 2 AI — per-topic writing with article content (parallel)
// Stage 5: Save + mark run success
// Stage 6: Generate thumbnail images (best-effort, does not affect run status)
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
		slog.Error("collection run failed", "run_id", run.ID, "error", err)
		errMsg := err.Error()
		_ = s.repo.CompleteRun(ctx, run.ID, RunStatusFailed, &errMsg, rawResp)
		return CollectionResult{}, err
	}

	// Stage 0: collect from structured sources (no AI involved).
	slog.Info("collection run stage 0: structured source collection", "run_id", run.ID)
	data, err := s.aggregator.Collect(ctx)
	if err != nil {
		return failRun(fmt.Errorf("source collection: %w", err), nil)
	}
	slog.Info("collection run stage 0 done", "run_id", run.ID, "items", len(data.Items))

	// Checkpoint: Stage 0 complete — save formatted text for resume.
	s.saveCheckpoint(ctx, run.ID, 0, Stage0Data{FormattedText: data.FormattedText})

	// Persist raw trending data for tracking/analysis.
	if err := s.trendingRepo.SaveTrendingItems(ctx, run.ID, data.Items); err != nil {
		slog.Warn("failed to save trending items", "error", err)
	}

	// Load brain categories for AI prompt injection.
	brainCategories, err := s.brainCatRepo.GetAll(ctx)
	if err != nil {
		slog.Warn("failed to load brain categories, proceeding without them", "error", err)
		brainCategories = nil
	}

	// Load categories for AI prompt injection.
	var categories []Category
	if s.catRepo != nil {
		categories, err = s.catRepo.GetAllCategories(ctx)
		if err != nil {
			slog.Warn("failed to load categories, using defaults", "error", err)
			categories = nil
		}
	}

	// Stage 1: Phase 1 AI — clustering + categorization + buzz_score + source selection.
	slog.Info("collection run stage 1: Phase 1 AI clustering", "run_id", run.ID)
	phase1Prompt := BuildClusterPrompt(data.FormattedText, brainCategories, categories)
	phase1Resp, err := s.callAIWithRetry(ctx, phase1Prompt)
	if err != nil {
		return failRun(fmt.Errorf("Phase 1 AI clustering: %w", err), nil)
	}

	topics, err := parsePhase1Response(phase1Resp.OutputText)
	if err != nil {
		rawResp := phase1Resp.RawJSON
		return failRun(fmt.Errorf("parsing Phase 1 response: %w", err), &rawResp)
	}
	slog.Info("collection run stage 1 done", "run_id", run.ID, "topics", len(topics))

	// Checkpoint: Stage 1 complete — save topics + raw Phase 1 JSON for resume.
	s.saveCheckpoint(ctx, run.ID, 1, Stage1Data{Topics: topics, Phase1RawJSON: phase1Resp.RawJSON})

	// Stage 2: Decode redirect URLs to original article URLs.
	topics = s.decodePhase1URLs(ctx, run.ID, topics)

	// Stage 3: Fetch article content + validate sources.
	// Blocked hosts and failed fetches are removed. Topics with zero valid sources are dropped.
	slog.Info("collection run stage 3: fetching articles + validating sources", "run_id", run.ID)
	topics, articleMap := s.fetchAndValidateSources(ctx, run.ID, topics)
	if len(topics) == 0 {
		rawResp := phase1Resp.RawJSON
		return failRun(fmt.Errorf("all topics dropped after source validation/fetch"), &rawResp)
	}
	slog.Info("collection run stage 3 done", "run_id", run.ID, "topics", len(topics))

	// Checkpoint: Stage 3 complete — save topics + articles + raw JSON for resume.
	s.saveCheckpoint(ctx, run.ID, 3, Stage3Data{Topics: topics, ArticleMap: articleMap, Phase1RawJSON: phase1Resp.RawJSON})

	// Stages 4-7: shared pipeline tail.
	return s.runPipelineTail(ctx, &run, topics, articleMap, brainCategories, phase1Resp.RawJSON, failRun)
}

// CollectFromSourcesIfNeeded checks if collection already ran today and skips if so.
// When a checkpoint repository is configured, it attempts to resume a failed run
// before starting a fresh one.
func (s *Service) CollectFromSourcesIfNeeded(ctx context.Context) (*CollectionResult, error) {
	canRun, err := s.repo.CanRunToday(ctx)
	if err != nil {
		return nil, fmt.Errorf("checking run status: %w", err)
	}

	if !canRun {
		slog.Info("collection skipped: already collected today")
		return nil, nil
	}

	result, err := s.ResumeOrCollect(ctx)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// ResumeOrCollect checks for a resumable failed run with checkpoint data.
// If found, creates a new run and resumes from the last completed stage.
// If not found (or checkpointRepo is nil), delegates to CollectFromSources.
func (s *Service) ResumeOrCollect(ctx context.Context) (CollectionResult, error) {
	if s.checkpointRepo == nil {
		return s.CollectFromSources(ctx)
	}

	oldRun, stage, cpData, err := s.checkpointRepo.GetLatestResumableRun(ctx, 3*time.Hour)
	if err != nil {
		slog.Warn("failed to check for resumable run, starting fresh", "error", err)
		return s.CollectFromSources(ctx)
	}
	if oldRun == nil {
		return s.CollectFromSources(ctx)
	}

	// Validate checkpoint envelope (version check).
	cpStage, innerData, err := unmarshalCheckpoint(cpData)
	if err != nil {
		slog.Warn("corrupt checkpoint data, starting fresh", "old_run_id", oldRun.ID, "error", err)
		return s.CollectFromSources(ctx)
	}
	if stage == nil || cpStage != *stage {
		slog.Warn("checkpoint stage mismatch, starting fresh", "old_run_id", oldRun.ID)
		return s.CollectFromSources(ctx)
	}

	slog.Info("resuming from checkpoint", "old_run_id", oldRun.ID, "stage", *stage)
	return s.resumeFromCheckpoint(ctx, *stage, innerData)
}

// resumeFromCheckpoint creates a new run and executes the pipeline from startAfterStage+1 onwards.
func (s *Service) resumeFromCheckpoint(ctx context.Context, startAfterStage int, data json.RawMessage) (CollectionResult, error) {
	run := CollectionRun{
		ID:        uuid.New(),
		StartedAt: time.Now().UTC(),
		Status:    RunStatusRunning,
	}

	// Atomic concurrency guard: only insert if no other run is currently running today.
	created, err := s.checkpointRepo.CreateRunIfIdle(ctx, run)
	if err != nil {
		return CollectionResult{}, fmt.Errorf("creating resume run: %w", err)
	}
	if !created {
		slog.Info("resume skipped: another run is already active")
		return CollectionResult{}, nil
	}

	failRun := func(err error, rawResp *string) (CollectionResult, error) {
		slog.Error("resumed collection run failed", "run_id", run.ID, "error", err)
		errMsg := err.Error()
		_ = s.repo.CompleteRun(ctx, run.ID, RunStatusFailed, &errMsg, rawResp)
		return CollectionResult{}, err
	}

	// Re-fetch brain categories from DB (not stored in checkpoint).
	// Needed by Stage 1 (BuildClusterPrompt) and Stage 4 (BuildDetailPrompt).
	brainCategories, err := s.brainCatRepo.GetAll(ctx)
	if err != nil {
		slog.Warn("failed to load brain categories on resume, proceeding without them", "error", err)
		brainCategories = nil
	}

	// abandonAndFresh marks the orphaned resume run as failed, then starts a fresh pipeline.
	abandonAndFresh := func(reason string) (CollectionResult, error) {
		slog.Warn(reason, "run_id", run.ID, "stage", startAfterStage)
		errMsg := reason
		_ = s.repo.CompleteRun(ctx, run.ID, RunStatusFailed, &errMsg, nil)
		return s.CollectFromSources(ctx)
	}

	// Deserialize checkpoint and execute remaining stages.
	var formattedText string
	var topics []Phase1Topic
	var articleMap map[int][]FetchedArticle
	var phase1RawJSON string

	switch startAfterStage {
	case 0:
		var cp Stage0Data
		if err := json.Unmarshal(data, &cp); err != nil {
			return abandonAndFresh("failed to deserialize Stage 0 checkpoint, starting fresh")
		}
		formattedText = cp.FormattedText

	case 1:
		var cp Stage1Data
		if err := json.Unmarshal(data, &cp); err != nil {
			return abandonAndFresh("failed to deserialize Stage 1 checkpoint, starting fresh")
		}
		topics = cp.Topics
		phase1RawJSON = cp.Phase1RawJSON

	case 3:
		var cp Stage3Data
		if err := json.Unmarshal(data, &cp); err != nil {
			return abandonAndFresh("failed to deserialize Stage 3 checkpoint, starting fresh")
		}
		topics = cp.Topics
		articleMap = cp.ArticleMap
		phase1RawJSON = cp.Phase1RawJSON

	default:
		return abandonAndFresh("unknown checkpoint stage, starting fresh")
	}

	// Execute remaining stages based on where we resume from.

	// Stage 1: Phase 1 AI clustering (only if resuming from Stage 0).
	if startAfterStage < 1 {
		// categories are only needed for BuildClusterPrompt (Stage 1).
		var categories []Category
		if s.catRepo != nil {
			categories, err = s.catRepo.GetAllCategories(ctx)
			if err != nil {
				slog.Warn("failed to load categories on resume, using defaults", "error", err)
				categories = nil
			}
		}

		slog.Info("resumed run stage 1: Phase 1 AI clustering", "run_id", run.ID)
		phase1Prompt := BuildClusterPrompt(formattedText, brainCategories, categories)
		phase1Resp, err := s.callAIWithRetry(ctx, phase1Prompt)
		if err != nil {
			return failRun(fmt.Errorf("Phase 1 AI clustering: %w", err), nil)
		}

		topics, err = parsePhase1Response(phase1Resp.OutputText)
		if err != nil {
			rawResp := phase1Resp.RawJSON
			return failRun(fmt.Errorf("parsing Phase 1 response: %w", err), &rawResp)
		}
		phase1RawJSON = phase1Resp.RawJSON
		slog.Info("resumed run stage 1 done", "run_id", run.ID, "topics", len(topics))

		s.saveCheckpoint(ctx, run.ID, 1, Stage1Data{Topics: topics, Phase1RawJSON: phase1RawJSON})
	}

	// Stages 2+3: URL decode + article fetch (only if resuming from before Stage 3).
	if startAfterStage < 3 {
		topics = s.decodePhase1URLs(ctx, run.ID, topics)

		slog.Info("resumed run stage 3: fetching articles + validating sources", "run_id", run.ID)
		topics, articleMap = s.fetchAndValidateSources(ctx, run.ID, topics)
		if len(topics) == 0 {
			return failRun(fmt.Errorf("all topics dropped after source validation/fetch"), &phase1RawJSON)
		}
		slog.Info("resumed run stage 3 done", "run_id", run.ID, "topics", len(topics))

		s.saveCheckpoint(ctx, run.ID, 3, Stage3Data{Topics: topics, ArticleMap: articleMap, Phase1RawJSON: phase1RawJSON})
	}

	// Stages 4-7: shared pipeline tail.
	return s.runPipelineTail(ctx, &run, topics, articleMap, brainCategories, phase1RawJSON, failRun)
}

// runPipelineTail executes Stages 4-7 of the collection pipeline.
// Shared by both CollectFromSources and resumeFromCheckpoint to avoid duplication.
func (s *Service) runPipelineTail(
	ctx context.Context,
	run *CollectionRun,
	topics []Phase1Topic,
	articleMap map[int][]FetchedArticle,
	brainCategories []BrainCategory,
	phase1RawJSON string,
	failRun func(error, *string) (CollectionResult, error),
) (CollectionResult, error) {
	// Stage 4: Phase 2 AI — per-topic detail writing (parallel with semaphore).
	slog.Info("collection run stage 4: Phase 2 AI detail writing", "run_id", run.ID)
	items, pollsByItem, phase2RawResponses := s.runPhase2(ctx, run.ID, topics, articleMap, brainCategories)
	if len(items) == 0 {
		return failRun(fmt.Errorf("Phase 2 AI: all topics failed"), &phase1RawJSON)
	}
	slog.Info("collection run stage 4 done", "run_id", run.ID, "written", len(items), "total", len(topics))

	// Build combined raw response for debugging (before save so it's available on failure).
	combinedRaw := buildCombinedRawResponse(phase1RawJSON, phase2RawResponses)

	// Stage 5: Save items + mark run as success.
	if err := s.repo.SaveContextItems(ctx, items); err != nil {
		return failRun(fmt.Errorf("saving context items: %w", err), &combinedRaw)
	}
	if err := s.repo.CompleteRun(ctx, run.ID, RunStatusSuccess, nil, &combinedRaw); err != nil {
		return CollectionResult{}, fmt.Errorf("completing run: %w", err)
	}
	s.clearCheckpoint(ctx, run.ID)
	slog.Info("collection run stage 5 done", "run_id", run.ID, "items_saved", len(items))

	// Persist opinion polls emitted inline by Phase 2 (best-effort).
	if s.pollRepo != nil && len(pollsByItem) > 0 {
		s.savePollsFromPhase2(ctx, run.ID, items, pollsByItem)
	}

	// Stage 6: Generate thumbnail images (best-effort, does NOT affect run status).
	items = s.generateImages(ctx, run.ID, items)
	s.persistImagePaths(ctx, run.ID, items)

	// Stage 7: Generate quizzes (best-effort, does NOT affect run status).
	if s.quizRepo != nil {
		s.generateQuizzes(ctx, run.ID, items)
	}

	slog.Info("collection run complete", "run_id", run.ID, "items", len(items))

	now := time.Now().UTC()
	run.CompletedAt = &now
	run.Status = RunStatusSuccess
	run.RawResponse = &combinedRaw
	return CollectionResult{Run: *run, Items: items}, nil
}

// saveCheckpoint persists intermediate pipeline data for resume capability.
// No-op when checkpointRepo is nil. Errors are logged but never abort the pipeline.
func (s *Service) saveCheckpoint(ctx context.Context, runID uuid.UUID, stage int, payload any) {
	if s.checkpointRepo == nil {
		return
	}
	data, err := marshalCheckpoint(stage, payload)
	if err != nil {
		slog.Warn("failed to marshal checkpoint", "run_id", runID, "stage", stage, "error", err)
		return
	}
	if err := s.checkpointRepo.SaveCheckpoint(ctx, runID, stage, data); err != nil {
		slog.Warn("failed to save checkpoint", "run_id", runID, "stage", stage, "error", err)
	}
}

// clearCheckpoint removes checkpoint data after a successful run.
func (s *Service) clearCheckpoint(ctx context.Context, runID uuid.UUID) {
	if s.checkpointRepo == nil {
		return
	}
	if err := s.checkpointRepo.ClearCheckpoint(ctx, runID); err != nil {
		slog.Warn("failed to clear checkpoint", "run_id", runID, "error", err)
	}
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
			slog.Warn("Phase 1: dropping invalid topic", "hint", t.TopicHint, "category", t.Category, "sources", len(t.Sources))
			continue
		}
		// Default priority to "none" if empty
		if t.Priority == "" {
			t.Priority = "none"
		}
		valid = append(valid, t)
	}

	if len(valid) == 0 {
		return nil, fmt.Errorf("no valid topics after filtering (raw count: %d)", len(payload.Topics))
	}

	for _, t := range valid {
		slog.Info("Phase 1 topic", "hint", t.TopicHint, "category", t.Category, "priority", t.Priority, "brain", t.BrainCategory, "buzz", t.BuzzScore, "sources", len(t.Sources))
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

// isGoogleNewsURL checks if a URL is a Google News redirect URL by parsing the hostname.
func isGoogleNewsURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "news.google.com")
}

func (s *Service) decodePhase1URLs(ctx context.Context, runID uuid.UUID, topics []Phase1Topic) []Phase1Topic {
	slog.Info("collection run stage 2: decoding redirect URLs", "run_id", runID)

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
	slog.Info("collection run stage 2 done", "run_id", runID, "decoded_urls", decoded, "topics", len(topics))

	// Filter out any Google News URLs that failed to decode (silent fallback from decoder).
	droppedTotal := 0
	for i := range result {
		var filtered []string
		for _, src := range result[i].Sources {
			if isGoogleNewsURL(src) {
				slog.Warn("stage 2: dropped undecoded Google News URL", "topic", result[i].TopicHint, "url", src)
				droppedTotal++
				continue
			}
			filtered = append(filtered, src)
		}
		result[i].Sources = filtered
	}
	if droppedTotal > 0 {
		slog.Warn("stage 2: dropped undecoded Google News URLs total", "run_id", runID, "count", droppedTotal)
	}

	return result
}

// --- Article fetching + source validation ---

// fetchAndValidateSources validates sources, fetches articles, and filters topics in one pass.
// Sources are checked in two phases:
//  1. Pre-fetch: blocked hosts (portals, aggregators) are removed
//  2. Post-fetch: HTTP errors and empty bodies are removed
//
// Topics that end up with zero valid sources are dropped entirely (no content = hallucination risk).
// Returns the surviving topics (re-indexed) and their fetched articles.
func (s *Service) fetchAndValidateSources(ctx context.Context, runID uuid.UUID, topics []Phase1Topic) ([]Phase1Topic, map[int][]FetchedArticle) {
	articleMap := make(map[int][]FetchedArticle, len(topics))
	var surviving []Phase1Topic

	for _, t := range topics {
		// Phase 1: remove blocked hosts before making any HTTP requests.
		var fetchable []string
		for _, src := range t.Sources {
			if reason := checkBlockedURL(src); reason != "" {
				slog.Warn("topic source blocked pre-fetch", "topic", t.TopicHint, "url", src, "reason", reason)
				continue
			}
			fetchable = append(fetchable, src)
		}
		if len(fetchable) == 0 {
			slog.Warn("collection run: dropped topic (all sources blocked)", "run_id", runID, "topic", t.TopicHint)
			continue
		}

		// Phase 2: fetch articles and keep only sources that returned content.
		articles := s.articleFetcher(ctx, fetchable)
		var validSources []string
		var validArticles []FetchedArticle
		for _, a := range articles {
			if a.Err != nil {
				slog.Warn("topic source fetch failed", "topic", t.TopicHint, "url", a.URL, "error", a.Err)
				continue
			}
			if a.Body == "" {
				slog.Warn("topic source fetch empty body", "topic", t.TopicHint, "url", a.URL)
				continue
			}
			validSources = append(validSources, a.URL)
			validArticles = append(validArticles, a)
		}

		if len(validSources) == 0 {
			slog.Warn("collection run: dropped topic (no content)", "run_id", runID, "topic", t.TopicHint)
			continue
		}

		slog.Info("topic sources validated", "topic", t.TopicHint, "valid", len(validSources), "total", len(t.Sources))

		newIdx := len(surviving)
		validated := t
		validated.Sources = validSources
		surviving = append(surviving, validated)
		articleMap[newIdx] = validArticles
	}

	return surviving, articleMap
}

// --- Phase 2 parallel execution ---

const phase2Concurrency = 5

func (s *Service) runPhase2(
	ctx context.Context,
	runID uuid.UUID,
	topics []Phase1Topic,
	articleMap map[int][]FetchedArticle,
	brainCategories []BrainCategory,
) ([]ContextItem, map[uuid.UUID]PollData, []string) {
	type phase2Output struct {
		index int
		item  ContextItem
		poll  *PollData
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
			// Sources are already validated in stage 3 — all articles here have content.

			prompt := BuildDetailPrompt(t, articles, brainCategories)

			resp, err := s.callAIWithRetry(ctx, prompt)
			if err != nil {
				slog.Warn("Phase 2 failed for topic", "run_id", runID, "topic", t.TopicHint, "error", err)
				results <- phase2Output{index: idx, err: err}
				return
			}

			p2Result, err := parsePhase2Response(resp.OutputText)
			if err != nil {
				slog.Warn("Phase 2 parse failed for topic", "run_id", runID, "topic", t.TopicHint, "error", err)
				results <- phase2Output{index: idx, err: err, raw: resp.RawJSON}
				return
			}

			item := ContextItem{
				ID:              uuid.New(),
				CollectionRunID: runID,
				Category:        t.Category,
				Priority:        t.Priority,
				BrainCategory:   t.BrainCategory,
				Rank:            rankForTopic(t, topics),
				Topic:           p2Result.Topic,
				Summary:         p2Result.Summary,
				Detail:          p2Result.Detail,
				Details:         p2Result.Details,
				BuzzScore:       t.BuzzScore,
				Sources:         t.Sources,
			}

			results <- phase2Output{index: idx, item: item, poll: p2Result.Poll, raw: resp.RawJSON}
		}(i, topic)
	}

	wg.Wait()
	close(results)

	var items []ContextItem
	polls := make(map[uuid.UUID]PollData)
	rawResponses := make([]string, 0, len(topics))
	for out := range results {
		if out.err != nil {
			if out.raw != "" {
				rawResponses = append(rawResponses, out.raw)
			}
			continue
		}
		items = append(items, out.item)
		if out.poll != nil {
			polls[out.item.ID] = *out.poll
		}
		rawResponses = append(rawResponses, out.raw)
	}

	return items, polls, rawResponses
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
		slog.Warn("primary AI exhausted retries with infrastructure error, switching to fallback model")
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
			slog.Error("AI error (non-retryable)", "model", label, "attempt", attempt, "max", maxRetries, "error", aiErr)
			return AIResponse{}, aiErr
		}

		if attempt == maxRetries {
			slog.Error("AI error (final attempt)", "model", label, "attempt", attempt, "max", maxRetries, "error", aiErr)
			break
		}

		slog.Warn("AI error, retrying", "model", label, "attempt", attempt, "max", maxRetries, "backoff", backoff, "error", aiErr)

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

// --- Stage helpers ---

// persistImagePaths updates each item's image_path in the DB after image generation.
// Errors are logged but do not affect the run status — images are best-effort.
func (s *Service) persistImagePaths(ctx context.Context, runID uuid.UUID, items []ContextItem) {
	var saved, failed int
	for _, item := range items {
		if item.ImagePath == nil {
			continue
		}
		if err := s.repo.UpdateItemImagePath(ctx, item.ID, *item.ImagePath); err != nil {
			slog.Warn("failed to persist image path", "run_id", runID, "item_id", item.ID, "topic", item.Topic, "error", err)
			failed++
			continue
		}
		saved++
	}
	if failed > 0 {
		errMsg := fmt.Sprintf("image path persist: %d/%d failed", failed, saved+failed)
		_ = s.repo.CompleteRun(ctx, runID, RunStatusSuccess, &errMsg, nil)
	}
	if saved > 0 {
		slog.Info("persisted image paths", "run_id", runID, "count", saved)
	}
}

// generateImages creates thumbnail images for each item using the image generator.
// Returns a new slice with ImagePath populated for items that succeeded.
func (s *Service) generateImages(ctx context.Context, runID uuid.UUID, items []ContextItem) []ContextItem {
	slog.Info("collection run stage 6: generating thumbnail images", "run_id", runID)

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

	slog.Info("collection run stage 6 done", "run_id", runID, "generated", generated, "total", len(items))
	return result
}

const quizConcurrency = 5

// generateQuizzes generates quiz questions for each item via a separate AI call.
// Runs after Stage 6, best-effort — failures are logged and do NOT affect run status.
func (s *Service) generateQuizzes(ctx context.Context, runID uuid.UUID, items []ContextItem) {
	slog.Info("collection run stage 7: generating quizzes", "run_id", runID, "items", len(items))

	type quizOutput struct {
		quiz quiz.Quiz
		err  error
	}

	results := make(chan quizOutput, len(items))
	sem := make(chan struct{}, quizConcurrency)
	var wg sync.WaitGroup

	for _, item := range items {
		wg.Add(1)
		go func(it ContextItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			prompt := BuildQuizPrompt(it.Topic, it.Summary, it.Detail, it.Details)
			resp, err := s.callAIWithRetry(ctx, prompt)
			if err != nil {
				slog.Warn("stage 7: quiz AI call failed", "run_id", runID, "item_id", it.ID, "topic", it.Topic, "error", err)
				results <- quizOutput{err: err}
				return
			}

			var qd QuizData
			cleanJSON := stripMarkdownCodeFence(resp.OutputText)
			if err := json.Unmarshal([]byte(cleanJSON), &qd); err != nil {
				slog.Warn("stage 7: quiz parse failed", "run_id", runID, "item_id", it.ID, "topic", it.Topic, "error", err)
				results <- quizOutput{err: err}
				return
			}

			if len(qd.Options) != 4 || qd.CorrectIndex < 0 || qd.CorrectIndex > 3 || qd.Question == "" {
				slog.Warn("stage 7: quiz validation failed", "run_id", runID, "item_id", it.ID, "topic", it.Topic,
					"options", len(qd.Options), "correct_index", qd.CorrectIndex)
				results <- quizOutput{err: fmt.Errorf("invalid quiz data")}
				return
			}

			results <- quizOutput{quiz: quiz.Quiz{
				ID:            uuid.New(),
				ContextItemID: it.ID,
				Question:      qd.Question,
				Options:       qd.Options,
				CorrectIndex:  qd.CorrectIndex,
			}}
		}(item)
	}

	wg.Wait()
	close(results)

	var quizzes []quiz.Quiz
	for out := range results {
		if out.err != nil {
			continue
		}
		quizzes = append(quizzes, out.quiz)
	}

	if len(quizzes) == 0 {
		slog.Warn("stage 7: no valid quizzes generated", "run_id", runID)
		return
	}

	if err := s.quizRepo.SaveQuizBatch(ctx, quizzes); err != nil {
		slog.Warn("stage 7: failed to save quizzes", "run_id", runID, "count", len(quizzes), "error", err)
		return
	}

	slog.Info("collection run stage 7 done", "run_id", runID, "saved", len(quizzes), "total", len(items))
}

// savePollsFromPhase2 persists polls emitted inline by Phase 2. Best-effort.
// Invalid payloads (empty strings, wrong option count) are logged and skipped.
func (s *Service) savePollsFromPhase2(ctx context.Context, runID uuid.UUID, items []ContextItem, pollsByItem map[uuid.UUID]PollData) {
	polls := make([]poll.Poll, 0, len(pollsByItem))
	skippedInvalid := 0
	for _, it := range items {
		pd, ok := pollsByItem[it.ID]
		if !ok {
			continue
		}
		if strings.TrimSpace(pd.Question) == "" || len(pd.Options) < 2 || len(pd.Options) > 4 {
			slog.Warn("poll validation failed",
				"run_id", runID, "item_id", it.ID, "options", len(pd.Options))
			skippedInvalid++
			continue
		}
		hasEmpty := false
		for _, o := range pd.Options {
			if strings.TrimSpace(o) == "" {
				hasEmpty = true
				break
			}
		}
		if hasEmpty {
			skippedInvalid++
			continue
		}
		polls = append(polls, poll.Poll{
			ID:            uuid.New(),
			ContextItemID: it.ID,
			Question:      pd.Question,
			Options:       pd.Options,
		})
	}
	if len(polls) == 0 {
		slog.Info("polls stage: nothing to save",
			"run_id", runID, "skipped_invalid", skippedInvalid, "phase2_poll_payloads", len(pollsByItem))
		return
	}
	if err := s.pollRepo.SavePollBatch(ctx, polls); err != nil {
		slog.Warn("polls stage: save failed",
			"run_id", runID, "count", len(polls), "error", err)
		return
	}
	slog.Info("polls stage: saved",
		"run_id", runID, "saved", len(polls), "skipped_invalid", skippedInvalid, "total_items", len(items))
}

