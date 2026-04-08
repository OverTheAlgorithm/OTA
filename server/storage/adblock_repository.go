package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AdblockRepository updates adblock detection timestamps on the users table.
type AdblockRepository struct {
	pool *pgxpool.Pool
}

// NewAdblockRepository constructs an AdblockRepository.
func NewAdblockRepository(pool *pgxpool.Pool) *AdblockRepository {
	return &AdblockRepository{pool: pool}
}

// UpdateAdblockStatus sets adblock_detected_at or adblock_not_detected_at to NOW()
// depending on the detected flag.
func (r *AdblockRepository) UpdateAdblockStatus(ctx context.Context, userID string, detected bool) error {
	var query string
	if detected {
		query = `UPDATE users SET adblock_detected_at = NOW() WHERE id = $1`
	} else {
		query = `UPDATE users SET adblock_not_detected_at = NOW() WHERE id = $1`
	}
	if _, err := r.pool.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("adblock: update status: %w", err)
	}
	return nil
}
