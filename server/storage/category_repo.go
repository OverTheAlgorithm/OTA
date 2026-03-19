package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"ota/domain/collector"
)

type CategoryRepository struct {
	pool *pgxpool.Pool
}

func NewCategoryRepository(pool *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{pool: pool}
}

func (r *CategoryRepository) GetAllCategories(ctx context.Context) ([]collector.Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT key, label, display_order FROM categories ORDER BY display_order`)
	if err != nil {
		return nil, fmt.Errorf("get all categories: %w", err)
	}
	defer rows.Close()

	var categories []collector.Category
	for rows.Next() {
		var c collector.Category
		if err := rows.Scan(&c.Key, &c.Label, &c.DisplayOrder); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

func (r *CategoryRepository) GetEnabledNewsSources(ctx context.Context) ([]collector.NewsSource, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, category_key, provider, url, enabled FROM news_sources WHERE enabled = true ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("get enabled news sources: %w", err)
	}
	defer rows.Close()

	var sources []collector.NewsSource
	for rows.Next() {
		var s collector.NewsSource
		if err := rows.Scan(&s.ID, &s.CategoryKey, &s.Provider, &s.URL, &s.Enabled); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}
