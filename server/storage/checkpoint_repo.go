package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/collector"
)

// CheckpointRepository implements collector.CheckpointRepository using PostgreSQL.
type CheckpointRepository struct {
	pool *pgxpool.Pool
}

func NewCheckpointRepository(pool *pgxpool.Pool) *CheckpointRepository {
	return &CheckpointRepository{pool: pool}
}

func (r *CheckpointRepository) SaveCheckpoint(ctx context.Context, runID uuid.UUID, stage int, data json.RawMessage) error {
	query := `UPDATE collection_runs SET last_completed_stage = $2, checkpoint_data = $3 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, runID, stage, data)
	if err != nil {
		return fmt.Errorf("saving checkpoint for run %s stage %d: %w", runID, stage, err)
	}
	return nil
}

func (r *CheckpointRepository) GetLatestResumableRun(ctx context.Context, maxAge time.Duration) (*collector.CollectionRun, *int, json.RawMessage, error) {
	cutoff := time.Now().UTC().Add(-maxAge)
	query := `
		SELECT id, started_at, status, last_completed_stage, checkpoint_data
		FROM collection_runs
		WHERE status = 'failed'
		  AND last_completed_stage IS NOT NULL
		  AND checkpoint_data IS NOT NULL
		  AND started_at > $1
		  AND DATE(started_at AT TIME ZONE 'Asia/Seoul') = DATE(NOW() AT TIME ZONE 'Asia/Seoul')
		ORDER BY started_at DESC
		LIMIT 1`

	var run collector.CollectionRun
	var stage int
	var data json.RawMessage

	err := r.pool.QueryRow(ctx, query, cutoff).Scan(
		&run.ID, &run.StartedAt, &run.Status, &stage, &data,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, fmt.Errorf("querying resumable run: %w", err)
	}

	return &run, &stage, data, nil
}

func (r *CheckpointRepository) ClearCheckpoint(ctx context.Context, runID uuid.UUID) error {
	query := `UPDATE collection_runs SET checkpoint_data = NULL, last_completed_stage = NULL WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, runID)
	if err != nil {
		return fmt.Errorf("clearing checkpoint for run %s: %w", runID, err)
	}
	return nil
}

// CreateRunIfIdle atomically inserts a new collection run only if no other run
// is currently in 'running' status for today (KST). Returns (true, nil) on success,
// (false, nil) if another run is already active (no error — caller should skip).
func (r *CheckpointRepository) CreateRunIfIdle(ctx context.Context, run collector.CollectionRun) (bool, error) {
	query := `
		INSERT INTO collection_runs (id, started_at, status)
		SELECT $1, $2, $3
		WHERE NOT EXISTS (
			SELECT 1 FROM collection_runs
			WHERE DATE(started_at AT TIME ZONE 'Asia/Seoul') = DATE(NOW() AT TIME ZONE 'Asia/Seoul')
			AND status = 'running'
		)`

	tag, err := r.pool.Exec(ctx, query, run.ID, run.StartedAt, string(run.Status))
	if err != nil {
		return false, fmt.Errorf("creating run if idle: %w", err)
	}

	return tag.RowsAffected() > 0, nil
}

// CleanupOldCheckpoints clears checkpoint data from runs older than 24 hours
// to prevent unbounded JSONB storage growth. Called at server startup.
func (r *CheckpointRepository) CleanupOldCheckpoints(ctx context.Context) (int, error) {
	query := `
		UPDATE collection_runs
		SET checkpoint_data = NULL, last_completed_stage = NULL
		WHERE checkpoint_data IS NOT NULL
		AND started_at < NOW() - INTERVAL '24 hours'`

	tag, err := r.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("cleaning up old checkpoints: %w", err)
	}

	return int(tag.RowsAffected()), nil
}
