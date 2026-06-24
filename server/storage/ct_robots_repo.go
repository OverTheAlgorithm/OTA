package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/communitytrend"
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

func (r *CTRobotsRepository) ListStatus(ctx context.Context) ([]communitytrend.RobotsStatus, error) {
	query := `
		SELECT DISTINCT ON (s.community_id)
		       s.community_id,
		       c.key,
		       c.name,
		       s.checked_at,
		       s.allowed,
		       s.snapshot_hash,
		       s.note
		FROM ct_robots_status s
		JOIN ct_communities c ON s.community_id = c.id
		ORDER BY s.community_id, s.checked_at DESC`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list robots status: %w", err)
	}
	defer rows.Close()

	var result []communitytrend.RobotsStatus
	for rows.Next() {
		var s communitytrend.RobotsStatus
		err := rows.Scan(
			&s.CommunityID, &s.CommunityKey, &s.CommunityName,
			&s.CheckedAt, &s.Allowed, &s.SnapshotHash, &s.Note,
		)
		if err != nil {
			return nil, fmt.Errorf("scan robots status: %w", err)
		}
		result = append(result, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate robots status: %w", err)
	}
	return result, nil
}

func (r *CTRobotsRepository) ListTransitions(ctx context.Context, limit int) ([]communitytrend.RobotsTransition, error) {
	query := `
		SELECT t.id,
		       t.community_id,
		       c.name,
		       t.from_allowed,
		       t.to_allowed,
		       t.changed_at
		FROM ct_robots_transitions t
		JOIN ct_communities c ON t.community_id = c.id
		ORDER BY t.changed_at DESC
		LIMIT $1`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list robots transitions: %w", err)
	}
	defer rows.Close()

	var result []communitytrend.RobotsTransition
	for rows.Next() {
		var t communitytrend.RobotsTransition
		err := rows.Scan(
			&t.ID, &t.CommunityID, &t.CommunityName,
			&t.FromAllowed, &t.ToAllowed, &t.ChangedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan robots transition: %w", err)
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate robots transitions: %w", err)
	}
	return result, nil
}
