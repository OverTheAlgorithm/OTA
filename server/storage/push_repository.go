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

// Save upserts a push token. On conflict (same device token), updates platform and user_id.
func (r *PushRepository) Save(ctx context.Context, t push.PushToken) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO push_tokens (id, user_id, token, platform)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (token) DO UPDATE SET platform = EXCLUDED.platform, user_id = EXCLUDED.user_id
	`, t.ID, t.UserID, t.Token, t.Platform)
	if err != nil {
		return fmt.Errorf("save push token: %w", err)
	}
	return nil
}

// UnlinkUser sets user_id to NULL for a token owned by the given user.
// The token row is preserved for anonymous push delivery.
func (r *PushRepository) UnlinkUser(ctx context.Context, userID, token string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE push_tokens SET user_id = NULL WHERE token = $1 AND user_id = $2
	`, token, userID)
	if err != nil {
		return fmt.Errorf("unlink push token: %w", err)
	}
	return nil
}

// DeleteByTokens removes push tokens matching any of the given token strings.
// Used to clean up stale tokens after Expo API reports DeviceNotRegistered.
func (r *PushRepository) DeleteByTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		DELETE FROM push_tokens WHERE token = ANY($1)
	`, tokens)
	if err != nil {
		return fmt.Errorf("delete stale push tokens: %w", err)
	}
	return nil
}

// GetByUserID returns all push tokens linked to a user.
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

// GetAllActive returns all push tokens (including anonymous).
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
