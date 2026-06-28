package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/communitytrend"
)

// CTAxisRepository implements communitytrend.AxisRepository.
type CTAxisRepository struct {
	pool *pgxpool.Pool
}

func NewCTAxisRepository(pool *pgxpool.Pool) *CTAxisRepository {
	return &CTAxisRepository{pool: pool}
}

func (r *CTAxisRepository) Create(ctx context.Context, a communitytrend.Axis) (communitytrend.Axis, error) {
	query := `
		INSERT INTO ct_axes (key, label, display_order, type)
		VALUES ($1, $2, $3, $4)
		RETURNING id, key, label, display_order, type`
	var out communitytrend.Axis
	err := r.pool.QueryRow(ctx, query, a.Key, a.Label, a.DisplayOrder, a.Type).Scan(
		&out.ID, &out.Key, &out.Label, &out.DisplayOrder, &out.Type,
	)
	if err != nil {
		return communitytrend.Axis{}, fmt.Errorf("create axis: %w", err)
	}
	return out, nil
}

func (r *CTAxisRepository) List(ctx context.Context) ([]communitytrend.Axis, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, key, label, display_order, type FROM ct_axes ORDER BY display_order, id`)
	if err != nil {
		return nil, fmt.Errorf("list axes: %w", err)
	}
	defer rows.Close()

	var result []communitytrend.Axis
	for rows.Next() {
		var a communitytrend.Axis
		if err := rows.Scan(&a.ID, &a.Key, &a.Label, &a.DisplayOrder, &a.Type); err != nil {
			return nil, fmt.Errorf("scan axis: %w", err)
		}
		result = append(result, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate axes: %w", err)
	}
	return result, nil
}

func (r *CTAxisRepository) Delete(ctx context.Context, id int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM ct_axes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete axis: %w", err)
	}
	return nil
}
