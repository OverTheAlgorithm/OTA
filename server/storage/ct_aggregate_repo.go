package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/communitytrend"
)

// CTAggregateRepository implements communitytrend.AggregateRepository.
type CTAggregateRepository struct {
	pool *pgxpool.Pool
}

func NewCTAggregateRepository(pool *pgxpool.Pool) *CTAggregateRepository {
	return &CTAggregateRepository{pool: pool}
}

func (r *CTAggregateRepository) scan(ctx context.Context, query string, args ...any) ([]communitytrend.DailyTagCount, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query aggregate: %w", err)
	}
	defer rows.Close()

	var out []communitytrend.DailyTagCount
	for rows.Next() {
		var c communitytrend.DailyTagCount
		if err := rows.Scan(&c.TagID, &c.TagName, &c.AxisKey, &c.StatDate, &c.PostCount); err != nil {
			return nil, fmt.Errorf("scan aggregate row: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate aggregate rows: %w", err)
	}
	return out, nil
}

// CommunitySeries returns daily tag counts for one community over [from,to].
func (r *CTAggregateRepository) CommunitySeries(ctx context.Context, communityID int, from, to time.Time) ([]communitytrend.DailyTagCount, error) {
	return r.scan(ctx, `
		SELECT td.tag_id, t.name, ax.key, td.stat_date, td.post_count
		FROM ct_tag_daily td
		JOIN ct_tags t  ON t.id = td.tag_id
		JOIN ct_axes ax ON ax.id = t.axis_id
		WHERE td.community_id = $1 AND td.stat_date BETWEEN $2 AND $3
		ORDER BY td.tag_id, td.stat_date`,
		communityID, from, to)
}

// CohortSeries sums daily tag counts across communities carrying the meta tag
// (the cohort dimension). The meta tag is matched on ct_community_tags.tag_id;
// the summed topic tag is ct_tag_daily.tag_id.
func (r *CTAggregateRepository) CohortSeries(ctx context.Context, metaTagID int, from, to time.Time) ([]communitytrend.DailyTagCount, error) {
	return r.scan(ctx, `
		SELECT td.tag_id, t.name, ax.key, td.stat_date, SUM(td.post_count)::int
		FROM ct_tag_daily td
		JOIN ct_community_tags cm ON cm.community_id = td.community_id AND cm.tag_id = $1
		JOIN ct_tags t  ON t.id = td.tag_id
		JOIN ct_axes ax ON ax.id = t.axis_id
		WHERE td.stat_date BETWEEN $2 AND $3
		GROUP BY td.tag_id, t.name, ax.key, td.stat_date
		ORDER BY td.tag_id, td.stat_date`,
		metaTagID, from, to)
}
