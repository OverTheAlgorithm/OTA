package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RefreshToken represents a stored refresh token record.
type RefreshToken struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// RefreshTokenRepository handles persistence of refresh tokens.
type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

// Insert stores a new hashed refresh token for the given user.
func (r *RefreshTokenRepository) Insert(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// FindByHash looks up a refresh token by its hash. Returns the record and
// whether it was found. Expired tokens are not returned.
func (r *RefreshTokenRepository) FindByHash(ctx context.Context, tokenHash string) (*RefreshToken, bool, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at
		 FROM refresh_tokens
		 WHERE token_hash = $1 AND expires_at > NOW()`,
		tokenHash,
	)

	var t RefreshToken
	err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("find refresh token: %w", err)
	}
	return &t, true, nil
}

// DeleteByHash removes a specific refresh token (used on rotation).
func (r *RefreshTokenRepository) DeleteByHash(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
	return nil
}

// DeleteAllForUser removes all refresh tokens for a user (logout / account deletion).
func (r *RefreshTokenRepository) DeleteAllForUser(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("delete all refresh tokens for user: %w", err)
	}
	return nil
}

// DeleteExpired purges all expired tokens — intended for periodic cleanup.
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE expires_at <= NOW()`,
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired refresh tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}
