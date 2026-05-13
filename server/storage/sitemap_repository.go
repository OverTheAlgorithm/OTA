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
}

// NewSitemapRepository constructs a SitemapRepository.
func NewSitemapRepository(pool *pgxpool.Pool) *SitemapRepository {
	return &SitemapRepository{pool: pool}
}

// GetAllTopicRows returns all topic IDs and creation timestamps ordered newest first.
func (r *SitemapRepository) GetAllTopicRows(ctx context.Context) ([]SitemapTopicRow, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text, created_at FROM context_items ORDER BY created_at DESC`)
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
