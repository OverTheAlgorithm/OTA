package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/communitytrend"
)

// CTCommunityRepository implements communitytrend.CommunityRepository.
type CTCommunityRepository struct {
	pool *pgxpool.Pool
}

func NewCTCommunityRepository(pool *pgxpool.Pool) *CTCommunityRepository {
	return &CTCommunityRepository{pool: pool}
}

func (r *CTCommunityRepository) Create(ctx context.Context, c communitytrend.Community) (communitytrend.Community, error) {
	query := `
		INSERT INTO ct_communities (key, name, home_url, enabled)
		VALUES ($1, $2, $3, $4)
		RETURNING id, key, name, home_url, enabled, created_at, updated_at`
	var out communitytrend.Community
	err := r.pool.QueryRow(ctx, query, c.Key, c.Name, c.HomeURL, c.Enabled).Scan(
		&out.ID, &out.Key, &out.Name, &out.HomeURL, &out.Enabled, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return communitytrend.Community{}, fmt.Errorf("create community: %w", err)
	}
	return out, nil
}

func (r *CTCommunityRepository) List(ctx context.Context) ([]communitytrend.Community, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, key, name, home_url, enabled, created_at, updated_at
		 FROM ct_communities ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list communities: %w", err)
	}
	defer rows.Close()

	var result []communitytrend.Community
	for rows.Next() {
		var c communitytrend.Community
		if err := rows.Scan(&c.ID, &c.Key, &c.Name, &c.HomeURL, &c.Enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan community: %w", err)
		}
		result = append(result, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate communities: %w", err)
	}
	return result, nil
}

func (r *CTCommunityRepository) Update(ctx context.Context, id int, name, homeURL string, enabled bool) (communitytrend.Community, error) {
	query := `
		UPDATE ct_communities SET name=$2, home_url=$3, enabled=$4, updated_at=now()
		WHERE id=$1
		RETURNING id, key, name, home_url, enabled, created_at, updated_at`
	var out communitytrend.Community
	err := r.pool.QueryRow(ctx, query, id, name, homeURL, enabled).Scan(
		&out.ID, &out.Key, &out.Name, &out.HomeURL, &out.Enabled, &out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return communitytrend.Community{}, fmt.Errorf("update community: %w", err)
	}
	return out, nil
}

func (r *CTCommunityRepository) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM ct_communities WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("delete community: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("community not found")
	}
	return nil
}

// SetMetaTags replaces the full meta-tag set atomically.
func (r *CTCommunityRepository) SetMetaTags(ctx context.Context, communityID int, tagIDs []int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM ct_community_tags WHERE community_id=$1`, communityID); err != nil {
		return fmt.Errorf("clear meta tags: %w", err)
	}
	for _, tagID := range tagIDs {
		if _, err := tx.Exec(ctx,
			`INSERT INTO ct_community_tags (community_id, tag_id) VALUES ($1, $2)`,
			communityID, tagID); err != nil {
			return fmt.Errorf("attach meta tag %d: %w", tagID, err)
		}
	}
	return tx.Commit(ctx)
}

func (r *CTCommunityRepository) GetMetaTags(ctx context.Context, communityID int) ([]int, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT tag_id FROM ct_community_tags WHERE community_id=$1 ORDER BY tag_id`, communityID)
	if err != nil {
		return nil, fmt.Errorf("get meta tags: %w", err)
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan meta tag id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate meta tags: %w", err)
	}
	return ids, nil
}
