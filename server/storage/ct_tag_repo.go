package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/communitytrend"
)

// CTTagRepository implements communitytrend.TagRepository.
type CTTagRepository struct {
	pool *pgxpool.Pool
}

func NewCTTagRepository(pool *pgxpool.Pool) *CTTagRepository {
	return &CTTagRepository{pool: pool}
}

const ctTagCols = `id, axis_id, name, description, created_by, created_at`

func (r *CTTagRepository) Create(ctx context.Context, t communitytrend.Tag) (communitytrend.Tag, error) {
	createdBy := t.CreatedBy
	if createdBy == "" {
		createdBy = "admin"
	}
	query := `
		INSERT INTO ct_tags (axis_id, name, description, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + ctTagCols
	var out communitytrend.Tag
	err := r.pool.QueryRow(ctx, query, t.AxisID, t.Name, t.Description, createdBy).Scan(
		&out.ID, &out.AxisID, &out.Name, &out.Description, &out.CreatedBy, &out.CreatedAt,
	)
	if err != nil {
		return communitytrend.Tag{}, fmt.Errorf("create tag: %w", err)
	}
	return out, nil
}

func (r *CTTagRepository) scanList(ctx context.Context, query string, args ...any) ([]communitytrend.Tag, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	var result []communitytrend.Tag
	for rows.Next() {
		var t communitytrend.Tag
		if err := rows.Scan(&t.ID, &t.AxisID, &t.Name, &t.Description, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tags: %w", err)
	}
	return result, nil
}

func (r *CTTagRepository) List(ctx context.Context) ([]communitytrend.Tag, error) {
	return r.scanList(ctx, `SELECT `+ctTagCols+` FROM ct_tags ORDER BY axis_id, name`)
}

func (r *CTTagRepository) ListByAxis(ctx context.Context, axisID int) ([]communitytrend.Tag, error) {
	return r.scanList(ctx, `SELECT `+ctTagCols+` FROM ct_tags WHERE axis_id=$1 ORDER BY name`, axisID)
}

func (r *CTTagRepository) Update(ctx context.Context, id int, name, description string) (communitytrend.Tag, error) {
	query := `UPDATE ct_tags SET name=$2, description=$3 WHERE id=$1 RETURNING ` + ctTagCols
	var out communitytrend.Tag
	err := r.pool.QueryRow(ctx, query, id, name, description).Scan(
		&out.ID, &out.AxisID, &out.Name, &out.Description, &out.CreatedBy, &out.CreatedAt,
	)
	if err != nil {
		return communitytrend.Tag{}, fmt.Errorf("update tag: %w", err)
	}
	return out, nil
}

func (r *CTTagRepository) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM ct_tags WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("tag not found")
	}
	return nil
}
