package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/collector"
)

type TrendingItemRepository struct {
	pool *pgxpool.Pool
}

func NewTrendingItemRepository(pool *pgxpool.Pool) *TrendingItemRepository {
	return &TrendingItemRepository{pool: pool}
}

func (r *TrendingItemRepository) SaveTrendingItems(ctx context.Context, runID uuid.UUID, items []collector.TrendingItem) error {
	query := `INSERT INTO trending_items (collection_run_id, keyword, source, traffic, category, article_urls, article_titles, published_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	for _, item := range items {
		urlsJSON, err := json.Marshal(item.ArticleURLs)
		if err != nil {
			return fmt.Errorf("marshaling article_urls for %s: %w", item.Keyword, err)
		}

		titlesJSON, err := json.Marshal(item.ArticleTitles)
		if err != nil {
			return fmt.Errorf("marshaling article_titles for %s: %w", item.Keyword, err)
		}

		_, err = r.pool.Exec(ctx, query,
			runID,
			item.Keyword,
			item.Source,
			item.Traffic,
			item.Category,
			urlsJSON,
			titlesJSON,
			item.PublishedAt,
		)
		if err != nil {
			return fmt.Errorf("inserting trending item %s: %w", item.Keyword, err)
		}
	}

	return nil
}

func (r *TrendingItemRepository) GetTrendingItemsByRunID(ctx context.Context, runID uuid.UUID) ([]collector.TrendingItem, error) {
	query := `SELECT keyword, source, traffic, category, article_urls, article_titles, published_at
		FROM trending_items
		WHERE collection_run_id = $1
		ORDER BY traffic DESC`

	rows, err := r.pool.Query(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("querying trending items: %w", err)
	}
	defer rows.Close()

	var items []collector.TrendingItem
	for rows.Next() {
		var item collector.TrendingItem
		var urlsJSON, titlesJSON []byte
		var category *string

		err := rows.Scan(
			&item.Keyword,
			&item.Source,
			&item.Traffic,
			&category,
			&urlsJSON,
			&titlesJSON,
			&item.PublishedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning trending item: %w", err)
		}

		if category != nil {
			item.Category = *category
		}
		if urlsJSON != nil {
			if err := json.Unmarshal(urlsJSON, &item.ArticleURLs); err != nil {
				return nil, fmt.Errorf("unmarshaling article_urls: %w", err)
			}
		}
		if titlesJSON != nil {
			if err := json.Unmarshal(titlesJSON, &item.ArticleTitles); err != nil {
				return nil, fmt.Errorf("unmarshaling article_titles: %w", err)
			}
		}

		items = append(items, item)
	}

	return items, rows.Err()
}
