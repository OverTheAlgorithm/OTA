package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/communitytrend"
)

// CTWorksheetRepository implements communitytrend.WorksheetRepository.
type CTWorksheetRepository struct {
	pool *pgxpool.Pool
}

func NewCTWorksheetRepository(pool *pgxpool.Pool) *CTWorksheetRepository {
	return &CTWorksheetRepository{pool: pool}
}

const ctWorksheetCols = `id, community_id, stat_date, mode, status, total_posts, confirmed_by, confirmed_at`

func scanWorksheet(row interface {
	Scan(dest ...any) error
}) (communitytrend.Worksheet, error) {
	var w communitytrend.Worksheet
	err := row.Scan(&w.ID, &w.CommunityID, &w.StatDate, &w.Mode, &w.Status, &w.TotalPosts, &w.ConfirmedBy, &w.ConfirmedAt)
	return w, err
}

func (r *CTWorksheetRepository) Ensure(ctx context.Context, communityID int, date time.Time, mode string) (communitytrend.Worksheet, error) {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO ct_worksheets (community_id, stat_date, mode)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (community_id, stat_date) DO NOTHING`,
		communityID, date, mode)
	if err != nil {
		return communitytrend.Worksheet{}, fmt.Errorf("ensure worksheet: %w", err)
	}
	row := r.pool.QueryRow(ctx,
		`SELECT `+ctWorksheetCols+` FROM ct_worksheets WHERE community_id=$1 AND stat_date=$2`,
		communityID, date)
	w, err := scanWorksheet(row)
	if err != nil {
		return communitytrend.Worksheet{}, fmt.Errorf("load worksheet: %w", err)
	}
	return w, nil
}

func (r *CTWorksheetRepository) ListByDate(ctx context.Context, date time.Time) ([]communitytrend.Worksheet, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+ctWorksheetCols+` FROM ct_worksheets WHERE stat_date=$1 ORDER BY community_id`, date)
	if err != nil {
		return nil, fmt.Errorf("list worksheets: %w", err)
	}
	defer rows.Close()

	var result []communitytrend.Worksheet
	for rows.Next() {
		w, err := scanWorksheet(rows)
		if err != nil {
			return nil, fmt.Errorf("scan worksheet: %w", err)
		}
		result = append(result, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate worksheets: %w", err)
	}
	return result, nil
}

// Confirm writes the full day's aggregate atomically (decisions.md D-001).
func (r *CTWorksheetRepository) Confirm(ctx context.Context, conf communitytrend.Confirmation) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. worksheet → confirmed (create if absent)
	if _, err := tx.Exec(ctx,
		`INSERT INTO ct_worksheets (community_id, stat_date, mode, status, total_posts, confirmed_by, confirmed_at)
		 VALUES ($1, $2, $3, 'confirmed', $4, $5, now())
		 ON CONFLICT (community_id, stat_date) DO UPDATE
		   SET status='confirmed', mode=$3, total_posts=$4, confirmed_by=$5, confirmed_at=now()`,
		conf.CommunityID, conf.StatDate, conf.Mode, conf.TotalPosts, conf.ConfirmedBy); err != nil {
		return fmt.Errorf("confirm worksheet: %w", err)
	}

	// 2. tag_daily upserts
	for _, c := range conf.Counts {
		if _, err := tx.Exec(ctx,
			`INSERT INTO ct_tag_daily (community_id, tag_id, stat_date, post_count, source, updated_at)
			 VALUES ($1, $2, $3, $4, $5, now())
			 ON CONFLICT (community_id, tag_id, stat_date) DO UPDATE
			   SET post_count=$4, source=$5, updated_at=now()`,
			conf.CommunityID, c.TagID, conf.StatDate, c.Count, conf.Source); err != nil {
			return fmt.Errorf("upsert tag_daily (tag %d): %w", c.TagID, err)
		}
	}

	// 3. community_daily total
	if _, err := tx.Exec(ctx,
		`INSERT INTO ct_community_daily (community_id, stat_date, total_posts)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (community_id, stat_date) DO UPDATE SET total_posts=$3`,
		conf.CommunityID, conf.StatDate, conf.TotalPosts); err != nil {
		return fmt.Errorf("upsert community_daily: %w", err)
	}

	// 4. seen fingerprints (auto path); manual path passes none
	for _, fp := range conf.Fingerprints {
		if _, err := tx.Exec(ctx,
			`INSERT INTO ct_seen_posts (community_id, fingerprint, first_seen)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (community_id, fingerprint) DO NOTHING`,
			conf.CommunityID, fp, conf.StatDate); err != nil {
			return fmt.Errorf("insert seen fingerprint: %w", err)
		}
	}

	return tx.Commit(ctx)
}
