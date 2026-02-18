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

type Service struct {
	ai   AIClient
	repo Repository
}

func NewService(aiClient AIClient, repo Repository) *Service {
	return &Service{
		ai:   aiClient,
		repo: repo,
	}
}

func (s *Service) CollectIfNeeded(ctx context.Context) (*CollectionResult, error) {
	canRun, err := s.repo.CanRunToday(ctx)
	if err != nil {
		return nil, fmt.Errorf("checking run status: %w", err)
	}

	if !canRun {
		return nil, nil
	}

	result, err := s.Collect(ctx)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *Service) Collect(ctx context.Context) (CollectionResult, error) {
	run := CollectionRun{
		ID:        uuid.New(),
		StartedAt: time.Now().UTC(),
		Status:    RunStatusRunning,
	}

	if err := s.repo.CreateRun(ctx, run); err != nil {
		return CollectionResult{}, fmt.Errorf("creating collection run: %w", err)
	}

	prompt := BuildCollectionPrompt()

	resp, err := s.callAIWithRetry(ctx, prompt)
	if err != nil {
		errMsg := err.Error()
		_ = s.repo.CompleteRun(ctx, run.ID, RunStatusFailed, &errMsg, nil)
		return CollectionResult{}, fmt.Errorf("collecting: %w", err)
	}

	items, err := parseContextItems(resp.OutputText, run.ID)
	if err != nil {
		errMsg := err.Error()
		rawResp := resp.RawJSON
		_ = s.repo.CompleteRun(ctx, run.ID, RunStatusFailed, &errMsg, &rawResp)
		return CollectionResult{}, fmt.Errorf("parsing ai response: %w", err)
	}

	if err := s.repo.SaveContextItems(ctx, items); err != nil {
		errMsg := err.Error()
		rawResp := resp.RawJSON
		_ = s.repo.CompleteRun(ctx, run.ID, RunStatusFailed, &errMsg, &rawResp)
		return CollectionResult{}, fmt.Errorf("saving context items: %w", err)
	}

	now := time.Now().UTC()
	rawResp := resp.RawJSON
	run.CompletedAt = &now
	run.Status = RunStatusSuccess
	run.RawResponse = &rawResp

	if err := s.repo.CompleteRun(ctx, run.ID, RunStatusSuccess, nil, &rawResp); err != nil {
		return CollectionResult{}, fmt.Errorf("completing run: %w", err)
	}

	return CollectionResult{Run: run, Items: items}, nil
}

const (
	maxRetries     = 3
	initialBackoff = 1 * time.Second
)

// callAIWithRetry calls the AI client with exponential backoff retry logic.
// Retries on network and infrastructure errors, but not on format errors.
func (s *Service) callAIWithRetry(ctx context.Context, prompt string) (AIResponse, error) {
	var lastErr error
	backoff := initialBackoff

	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := s.ai.SearchAndAnalyze(ctx, prompt)
		if err == nil {
			return resp, nil
		}

		// Classify the error
		aiErr := ClassifyError(err)
		lastErr = aiErr

		// Don't retry format errors - they won't be fixed by retrying
		if !aiErr.IsRetryable() {
			log.Printf("AI client error (non-retryable, attempt %d/%d): %v", attempt, maxRetries, aiErr)
			return AIResponse{}, aiErr
		}

		// Last attempt - don't wait
		if attempt == maxRetries {
			log.Printf("AI client error (final attempt %d/%d): %v", attempt, maxRetries, aiErr)
			break
		}

		// Log and wait before retry
		log.Printf("AI client error (attempt %d/%d, retrying in %v): %v", attempt, maxRetries, backoff, aiErr)

		select {
		case <-ctx.Done():
			return AIResponse{}, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(backoff):
			// Exponential backoff: double the wait time
			backoff *= 2
		}
	}

	return AIResponse{}, fmt.Errorf("AI client failed after %d attempts: %w", maxRetries, lastErr)
}

type aiResponsePayload struct {
	Items []aiContextItem `json:"items"`
}

type aiContextItem struct {
	Category string   `json:"category"`
	Rank     int      `json:"rank"`
	Topic    string   `json:"topic"`
	Summary  string   `json:"summary"`
	Sources  []string `json:"sources"`
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
			Rank:            raw.Rank,
			Topic:           raw.Topic,
			Summary:         raw.Summary,
			Sources:         raw.Sources,
		})
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no valid context items after filtering")
	}

	return items, nil
}
