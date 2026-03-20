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
	query := `INSERT INTO context_items (id, collection_run_id, category, brain_category, rank, topic, summary, detail, details, buzz_score, sources, image_path, priority) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

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

		priority := item.Priority
		if priority == "" {
			priority = "none"
		}

		_, err = tx.Exec(ctx, query, item.ID, item.CollectionRunID, item.Category, brainCat, item.Rank, item.Topic, item.Summary, item.Detail, detailsJSON, item.BuzzScore, sourcesJSON, item.ImagePath, priority)
		if err != nil {
			return fmt.Errorf("inserting context item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (r *CollectorRepository) UpdateItemImagePath(ctx context.Context, itemID uuid.UUID, imagePath string) error {
	query := `UPDATE context_items SET image_path = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, itemID, imagePath)
	if err != nil {
		return fmt.Errorf("updating image path for item %s: %w", itemID, err)
	}
	return nil
}

func (r *CollectorRepository) CanRunToday(ctx context.Context) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM collection_runs
			WHERE DATE(started_at AT TIME ZONE 'Asia/Seoul') = DATE(NOW() AT TIME ZONE 'Asia/Seoul')
			AND (status = 'running' OR status = 'success')
		)`

	var exists bool
	err := r.pool.QueryRow(ctx, query).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking today's run status: %w", err)
	}

	return !exists, nil
}
