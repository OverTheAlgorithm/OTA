package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/collector"
)

// CollectorServiceAdapter adapts the storage layer to delivery.CollectorService interface
type CollectorServiceAdapter struct {
	pool *pgxpool.Pool
}

// NewCollectorServiceAdapter creates a new adapter
func NewCollectorServiceAdapter(pool *pgxpool.Pool) *CollectorServiceAdapter {
	return &CollectorServiceAdapter{pool: pool}
}

// GetLatestRun retrieves the most recent collection run
func (a *CollectorServiceAdapter) GetLatestRun(ctx context.Context) (*collector.CollectionRun, error) {
	query := `
		SELECT id, started_at, completed_at, status, error_message, raw_response
		FROM collection_runs
		ORDER BY started_at DESC
		LIMIT 1
	`

	var run collector.CollectionRun
	err := a.pool.QueryRow(ctx, query).Scan(
		&run.ID,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Status,
		&run.ErrorMessage,
		&run.RawResponse,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get latest run: %w", err)
	}

	return &run, nil
}

// GetLastDeliveredRun retrieves the most recent collection run that was already delivered to users
func (a *CollectorServiceAdapter) GetLastDeliveredRun(ctx context.Context) (*collector.CollectionRun, error) {
	query := `
		SELECT cr.id, cr.started_at, cr.completed_at, cr.status, cr.error_message, cr.raw_response
		FROM collection_runs cr
		WHERE EXISTS (
			SELECT 1 FROM delivery_logs dl
			WHERE dl.run_id = cr.id AND dl.status = 'sent'
		)
		ORDER BY cr.started_at DESC
		LIMIT 1
	`

	var run collector.CollectionRun
	err := a.pool.QueryRow(ctx, query).Scan(
		&run.ID,
		&run.StartedAt,
		&run.CompletedAt,
		&run.Status,
		&run.ErrorMessage,
		&run.RawResponse,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get last delivered run: %w", err)
	}

	return &run, nil
}

// GetContextItems retrieves all context items for a given run
func (a *CollectorServiceAdapter) GetContextItems(ctx context.Context, runID uuid.UUID) ([]collector.ContextItem, error) {
	query := `
		SELECT id, collection_run_id, category, rank, topic, summary,
		       COALESCE(detail, ''), COALESCE(details, '[]'), COALESCE(buzz_score, 0), COALESCE(sources, '{}')
		FROM context_items
		WHERE collection_run_id = $1
		ORDER BY rank
	`

	rows, err := a.pool.Query(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query context items: %w", err)
	}
	defer rows.Close()

	var items []collector.ContextItem
	for rows.Next() {
		var item collector.ContextItem
		var detailsJSON []byte
		err := rows.Scan(
			&item.ID,
			&item.CollectionRunID,
			&item.Category,
			&item.Rank,
			&item.Topic,
			&item.Summary,
			&item.Detail,
			&detailsJSON,
			&item.BuzzScore,
			&item.Sources,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan context item: %w", err)
		}
		_ = json.Unmarshal(detailsJSON, &item.Details)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating context items: %w", err)
	}

	return items, nil
}
