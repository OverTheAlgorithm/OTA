package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/user"
)

type EmailVerificationRepository struct {
	pool *pgxpool.Pool
}

func NewEmailVerificationRepository(pool *pgxpool.Pool) *EmailVerificationRepository {
	return &EmailVerificationRepository{pool: pool}
}

func (r *EmailVerificationRepository) CreateCode(ctx context.Context, code user.EmailVerificationCode) error {
	query := `
		INSERT INTO email_verification_codes (id, user_id, email, code, expires_at, used, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.pool.Exec(ctx, query,
		code.ID, code.UserID, code.Email, code.Code,
		code.ExpiresAt, code.Used, code.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create verification code: %w", err)
	}
	return nil
}

func (r *EmailVerificationRepository) FindValidCode(ctx context.Context, userID string, code string) (user.EmailVerificationCode, error) {
	query := `
		SELECT id, user_id, email, code, expires_at, used, created_at
		FROM email_verification_codes
		WHERE user_id = $1 AND code = $2 AND used = false AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`
	var vc user.EmailVerificationCode
	err := r.pool.QueryRow(ctx, query, userID, code).Scan(
		&vc.ID, &vc.UserID, &vc.Email, &vc.Code,
		&vc.ExpiresAt, &vc.Used, &vc.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return user.EmailVerificationCode{}, fmt.Errorf("no valid code found")
		}
		return user.EmailVerificationCode{}, fmt.Errorf("failed to find code: %w", err)
	}
	return vc, nil
}

func (r *EmailVerificationRepository) MarkCodeUsed(ctx context.Context, codeID string) error {
	query := `UPDATE email_verification_codes SET used = true WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, codeID)
	if err != nil {
		return fmt.Errorf("failed to mark code used: %w", err)
	}
	return nil
}

func (r *EmailVerificationRepository) CountRecentCodes(ctx context.Context, userID string, since time.Time) (int, error) {
	query := `SELECT COUNT(*) FROM email_verification_codes WHERE user_id = $1 AND created_at >= $2`
	var count int
	err := r.pool.QueryRow(ctx, query, userID, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count recent codes: %w", err)
	}
	return count, nil
}

func (r *EmailVerificationRepository) InvalidatePendingCodes(ctx context.Context, userID string) error {
	query := `UPDATE email_verification_codes SET used = true WHERE user_id = $1 AND used = false`
	_, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to invalidate pending codes: %w", err)
	}
	return nil
}
