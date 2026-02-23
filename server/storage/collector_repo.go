package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/collector"
)

type CollectorRepository struct {
	pool *pgxpool.Pool
}

func NewCollectorRepository(pool *pgxpool.Pool) *CollectorRepository {
	return &CollectorRepository{pool: pool}
}

func (r *CollectorRepository) CreateRun(ctx context.Context, run collector.CollectionRun) error {
	query := `INSERT INTO collection_runs (id, started_at, status) VALUES ($1, $2, $3)`

	_, err := r.pool.Exec(ctx, query, run.ID, run.StartedAt, string(run.Status))
	if err != nil {
		return fmt.Errorf("inserting collection run: %w", err)
	}
	return nil
}

func (r *CollectorRepository) CompleteRun(ctx context.Context, id uuid.UUID, status collector.RunStatus, errMsg *string, rawResponse *string) error {
	query := `UPDATE collection_runs SET completed_at = NOW(), status = $2, error_message = $3, raw_response = $4 WHERE id = $1`

	_, err := r.pool.Exec(ctx, query, id, string(status), errMsg, rawResponse)
	if err != nil {
		return fmt.Errorf("completing collection run: %w", err)
	}
	return nil
}

func (r *CollectorRepository) SaveContextItems(ctx context.Context, items []collector.ContextItem) error {
	query := `INSERT INTO context_items (id, collection_run_id, category, brain_category, rank, topic, summary, detail, details, buzz_score, sources) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	for _, item := range items {
		sourcesJSON, err := json.Marshal(item.Sources)
		if err != nil {
			return fmt.Errorf("marshaling sources for item %s: %w", item.Topic, err)
		}

		detailsJSON, err := json.Marshal(item.Details)
		if err != nil {
			return fmt.Errorf("marshaling details for item %s: %w", item.Topic, err)
		}

		var brainCat *string
		if item.BrainCategory != "" {
			brainCat = &item.BrainCategory
		}

		_, err = r.pool.Exec(ctx, query, item.ID, item.CollectionRunID, item.Category, brainCat, item.Rank, item.Topic, item.Summary, item.Detail, detailsJSON, item.BuzzScore, sourcesJSON)
		if err != nil {
			return fmt.Errorf("inserting context item: %w", err)
		}
	}

	return nil
}

func (r *CollectorRepository) CanRunToday(ctx context.Context) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM collection_runs
			WHERE DATE(started_at AT TIME ZONE 'UTC') = CURRENT_DATE
			AND (status = 'running' OR status = 'success')
		)`

	var exists bool
	err := r.pool.QueryRow(ctx, query).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking today's run status: %w", err)
	}

	return !exists, nil
}
