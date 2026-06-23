package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CTSeenRepository implements communitytrend.SeenRepository.
type CTSeenRepository struct {
	pool *pgxpool.Pool
}

func NewCTSeenRepository(pool *pgxpool.Pool) *CTSeenRepository {
	return &CTSeenRepository{pool: pool}
}

// LoadSeen returns the set of fingerprints already counted for a community.
func (r *CTSeenRepository) LoadSeen(ctx context.Context, communityID int) (map[string]bool, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT fingerprint FROM ct_seen_posts WHERE community_id=$1`, communityID)
	if err != nil {
		return nil, fmt.Errorf("load seen: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	for rows.Next() {
		var fp string
		if err := rows.Scan(&fp); err != nil {
			return nil, fmt.Errorf("scan fingerprint: %w", err)
		}
		seen[fp] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate fingerprints: %w", err)
	}
	return seen, nil
}

// Prune removes seen rows whose first_seen predates the cutoff.
func (r *CTSeenRepository) Prune(ctx context.Context, before time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM ct_seen_posts WHERE first_seen < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("prune seen: %w", err)
	}
	return tag.RowsAffected(), nil
}
