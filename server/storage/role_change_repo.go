package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/user"
)

// RoleChangeRepository persists role transitions for audit purposes.
type RoleChangeRepository struct {
	pool *pgxpool.Pool
}

func NewRoleChangeRepository(pool *pgxpool.Pool) *RoleChangeRepository {
	return &RoleChangeRepository{pool: pool}
}

func (r *RoleChangeRepository) Log(ctx context.Context, entry user.RoleChangeLog) (user.RoleChangeLog, error) {
	const query = `
		INSERT INTO role_change_logs (user_id, before_role, after_role, actor_id, memo)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, before_role, after_role, actor_id, memo, created_at`

	var result user.RoleChangeLog
	err := r.pool.QueryRow(ctx, query, entry.UserID, entry.BeforeRole, entry.AfterRole, entry.ActorID, entry.Memo).Scan(
		&result.ID, &result.UserID, &result.BeforeRole, &result.AfterRole, &result.ActorID, &result.Memo, &result.CreatedAt,
	)
	if err != nil {
		return user.RoleChangeLog{}, fmt.Errorf("insert role_change_log: %w", err)
	}
	return result, nil
}

func (r *RoleChangeRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]user.RoleChangeLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	const query = `
		SELECT id, user_id, before_role, after_role, actor_id, memo, created_at
		FROM role_change_logs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query role_change_logs: %w", err)
	}
	defer rows.Close()

	var result []user.RoleChangeLog
	for rows.Next() {
		var entry user.RoleChangeLog
		if err := rows.Scan(&entry.ID, &entry.UserID, &entry.BeforeRole, &entry.AfterRole, &entry.ActorID, &entry.Memo, &entry.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan role_change_log: %w", err)
		}
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate role_change_logs: %w", err)
	}
	return result, nil
}
