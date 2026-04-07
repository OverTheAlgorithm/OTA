package storage

import (
	"context"
	"fmt"

	"ota/domain/push"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PushRepository implements push.Repository using pgxpool.
type PushRepository struct {
	pool *pgxpool.Pool
}

func NewPushRepository(pool *pgxpool.Pool) *PushRepository {
	return &PushRepository{pool: pool}
}

// Save inserts a push token. Silently ignores duplicates (user_id, token).
func (r *PushRepository) Save(ctx context.Context, t push.PushToken) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO push_tokens (id, user_id, token, platform)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, token) DO NOTHING
	`, t.ID, t.UserID, t.Token, t.Platform)
	if err != nil {
		return fmt.Errorf("save push token: %w", err)
	}
	return nil
}

// Delete removes a specific push token for a user.
func (r *PushRepository) Delete(ctx context.Context, userID, token string) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM push_tokens WHERE user_id = $1 AND token = $2
	`, userID, token)
	if err != nil {
		return fmt.Errorf("delete push token: %w", err)
	}
	return nil
}

// GetByUserID returns all push tokens registered for a user.
func (r *PushRepository) GetByUserID(ctx context.Context, userID string) ([]push.PushToken, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, token, platform, created_at
		FROM push_tokens WHERE user_id = $1
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get push tokens by user: %w", err)
	}
	defer rows.Close()

	var tokens []push.PushToken
	for rows.Next() {
		var t push.PushToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Token, &t.Platform, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan push token: %w", err)
		}
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate push tokens: %w", err)
	}
	return tokens, nil
}

// GetAllActive returns all push tokens across all users.
func (r *PushRepository) GetAllActive(ctx context.Context) ([]push.PushToken, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, token, platform, created_at
		FROM push_tokens
	`)
	if err != nil {
		return nil, fmt.Errorf("get all push tokens: %w", err)
	}
	defer rows.Close()

	var tokens []push.PushToken
	for rows.Next() {
		var t push.PushToken
		if err := rows.Scan(&t.ID, &t.UserID, &t.Token, &t.Platform, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan push token: %w", err)
		}
		tokens = append(tokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate push tokens: %w", err)
	}
	return tokens, nil
}
