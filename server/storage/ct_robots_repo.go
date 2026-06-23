package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CTRobotsRepository implements communitytrend.RobotsRepository.
type CTRobotsRepository struct {
	pool *pgxpool.Pool
}

func NewCTRobotsRepository(pool *pgxpool.Pool) *CTRobotsRepository {
	return &CTRobotsRepository{pool: pool}
}

// Record stores a status sample and records a transition when the allowance
// changes from the most recent prior sample (or on the first sample).
func (r *CTRobotsRepository) Record(ctx context.Context, communityID int, allowed bool, snapshotHash, note string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var prior bool
	var priorFound bool
	err = tx.QueryRow(ctx,
		`SELECT allowed FROM ct_robots_status WHERE community_id=$1 ORDER BY checked_at DESC LIMIT 1`,
		communityID).Scan(&prior)
	switch {
	case err == nil:
		priorFound = true
	case errors.Is(err, pgx.ErrNoRows):
		priorFound = false
	default:
		return false, fmt.Errorf("read latest robots status: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO ct_robots_status (community_id, allowed, snapshot_hash, note)
		 VALUES ($1, $2, $3, $4)`,
		communityID, allowed, snapshotHash, note); err != nil {
		return false, fmt.Errorf("insert robots status: %w", err)
	}

	changed := !priorFound || prior != allowed
	if changed {
		var from *bool
		if priorFound {
			from = &prior
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO ct_robots_transitions (community_id, from_allowed, to_allowed)
			 VALUES ($1, $2, $3)`,
			communityID, from, allowed); err != nil {
			return false, fmt.Errorf("insert robots transition: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit robots record: %w", err)
	}
	return changed, nil
}

// LatestAllowed returns the most recent allowance for a community.
func (r *CTRobotsRepository) LatestAllowed(ctx context.Context, communityID int) (bool, bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx,
		`SELECT allowed FROM ct_robots_status WHERE community_id=$1 ORDER BY checked_at DESC LIMIT 1`,
		communityID).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("latest allowed: %w", err)
	}
	return allowed, true, nil
}
