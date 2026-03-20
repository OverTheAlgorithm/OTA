package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/collector"
)

type BrainCategoryRepository struct {
	pool *pgxpool.Pool
}

func NewBrainCategoryRepository(pool *pgxpool.Pool) *BrainCategoryRepository {
	return &BrainCategoryRepository{pool: pool}
}

func (r *BrainCategoryRepository) GetAll(ctx context.Context) ([]collector.BrainCategory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT key, emoji, label, accent_color, display_order, instruction, created_at, updated_at
		FROM brain_categories
		ORDER BY display_order, key
	`)
	if err != nil {
		return nil, fmt.Errorf("querying brain categories: %w", err)
	}
	defer rows.Close()

	var categories []collector.BrainCategory
	for rows.Next() {
		var bc collector.BrainCategory
		if err := rows.Scan(&bc.Key, &bc.Emoji, &bc.Label, &bc.AccentColor, &bc.DisplayOrder, &bc.Instruction, &bc.CreatedAt, &bc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning brain category: %w", err)
		}
		categories = append(categories, bc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate brain categories: %w", err)
	}
	if categories == nil {
		categories = []collector.BrainCategory{}
	}
	return categories, nil
}

func (r *BrainCategoryRepository) Create(ctx context.Context, bc collector.BrainCategory) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO brain_categories (key, emoji, label, accent_color, display_order, instruction)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, bc.Key, bc.Emoji, bc.Label, bc.AccentColor, bc.DisplayOrder, bc.Instruction)
	if err != nil {
		return fmt.Errorf("creating brain category: %w", err)
	}
	return nil
}

func (r *BrainCategoryRepository) Update(ctx context.Context, bc collector.BrainCategory) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE brain_categories
		SET emoji = $2, label = $3, accent_color = $4, display_order = $5, instruction = $6, updated_at = now()
		WHERE key = $1
	`, bc.Key, bc.Emoji, bc.Label, bc.AccentColor, bc.DisplayOrder, bc.Instruction)
	if err != nil {
		return fmt.Errorf("updating brain category: %w", err)
	}
	return nil
}

func (r *BrainCategoryRepository) Delete(ctx context.Context, key string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM brain_categories WHERE key = $1`, key)
	if err != nil {
		return fmt.Errorf("deleting brain category: %w", err)
	}
	return nil
}
