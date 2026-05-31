package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SitemapTopicRow is a raw row returned by SitemapRepository.
type SitemapTopicRow struct {
	ID        string
	CreatedAt time.Time
}

// SitemapRepository fetches topic data for sitemap generation.
type SitemapRepository struct {
	pool *pgxpool.Pool
	// minRefs filters sitemap entries so SEO crawlers do not pick up
	// single-source topics. Pass 0 to disable the filter (tests).
	minRefs int
}

// NewSitemapRepository constructs a SitemapRepository.
// minRefs filters topic rows by jsonb_array_length(sources) >= minRefs.
func NewSitemapRepository(pool *pgxpool.Pool, minRefs int) *SitemapRepository {
	if minRefs < 0 {
		minRefs = 0
	}
	return &SitemapRepository{pool: pool, minRefs: minRefs}
}

// GetAllTopicRows returns all topic IDs and creation timestamps ordered newest first.
// Topics with fewer than minRefs sources are excluded so they never appear in
// sitemap.xml and thus stay out of search engine indexes.
func (r *SitemapRepository) GetAllTopicRows(ctx context.Context) ([]SitemapTopicRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, created_at
		FROM context_items
		WHERE jsonb_array_length(COALESCE(sources, '[]'::jsonb)) >= $1
		ORDER BY created_at DESC
	`, r.minRefs)
	if err != nil {
		return nil, fmt.Errorf("sitemap: query topic IDs: %w", err)
	}
	defer rows.Close()

	var entries []SitemapTopicRow
	for rows.Next() {
		var e SitemapTopicRow
		if err := rows.Scan(&e.ID, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("sitemap: scan topic row: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sitemap: rows error: %w", err)
	}

	return entries, nil
}

// SitemapEditorPostRow holds the data needed to render an editor_pick entry.
type SitemapEditorPostRow struct {
	ID        string
	UpdatedAt time.Time
}

// GetAllEditorPostRows returns published editor posts ordered newest first.
func (r *SitemapRepository) GetAllEditorPostRows(ctx context.Context) ([]SitemapEditorPostRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id::text, updated_at FROM editor_posts
		 WHERE status = 'published' AND published_at IS NOT NULL
		 ORDER BY published_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("sitemap: query editor_post IDs: %w", err)
	}
	defer rows.Close()

	var entries []SitemapEditorPostRow
	for rows.Next() {
		var e SitemapEditorPostRow
		if err := rows.Scan(&e.ID, &e.UpdatedAt); err != nil {
			return nil, fmt.Errorf("sitemap: scan editor_post row: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sitemap: editor_post rows error: %w", err)
	}

	return entries, nil
}
