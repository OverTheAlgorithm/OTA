package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CleanupRepository handles data retention by deleting stale records.
type CleanupRepository struct {
	pool *pgxpool.Pool
}

func NewCleanupRepository(pool *pgxpool.Pool) *CleanupRepository {
	return &CleanupRepository{pool: pool}
}

// DeleteExpiredTrendingItems removes trending_items whose expires_at is in the past.
func (r *CleanupRepository) DeleteExpiredTrendingItems(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM trending_items WHERE expires_at < NOW()`,
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired trending items: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteOldCollectionRuns removes collection_runs (and cascaded context_items) older than the given age.
func (r *CleanupRepository) DeleteOldCollectionRuns(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM collection_runs WHERE completed_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("delete old collection runs: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteOldDeliveryLogs removes delivery_logs older than the given age.
func (r *CleanupRepository) DeleteOldDeliveryLogs(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM delivery_logs WHERE created_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("delete old delivery logs: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteExpiredRefreshTokens removes refresh_tokens whose expires_at is in the past.
func (r *CleanupRepository) DeleteExpiredRefreshTokens(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE expires_at < NOW()`,
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired refresh tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}

// RunAll runs all cleanup tasks and logs results. Errors from individual tasks are
// logged but do not prevent remaining tasks from running.
func (r *CleanupRepository) RunAll(ctx context.Context) {
	const retentionPeriod = 30 * 24 * time.Hour // 30 days

	n, err := r.DeleteExpiredTrendingItems(ctx)
	if err != nil {
		log.Printf("cleanup: delete expired trending items failed: %v", err)
	} else {
		log.Printf("cleanup: deleted %d expired trending items", n)
	}

	n, err = r.DeleteOldCollectionRuns(ctx, retentionPeriod)
	if err != nil {
		log.Printf("cleanup: delete old collection runs failed: %v", err)
	} else {
		log.Printf("cleanup: deleted %d collection runs older than 30 days", n)
	}

	n, err = r.DeleteOldDeliveryLogs(ctx, retentionPeriod)
	if err != nil {
		log.Printf("cleanup: delete old delivery logs failed: %v", err)
	} else {
		log.Printf("cleanup: deleted %d delivery logs older than 30 days", n)
	}

	n, err = r.DeleteExpiredRefreshTokens(ctx)
	if err != nil {
		log.Printf("cleanup: delete expired refresh tokens failed: %v", err)
	} else {
		log.Printf("cleanup: deleted %d expired refresh tokens", n)
	}
}
