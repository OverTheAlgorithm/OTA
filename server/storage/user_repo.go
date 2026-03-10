package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/user"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) UpsertByKakaoID(ctx context.Context, kakaoID int64, email, nickname, profileImage string) (user.User, error) {
	query := `
		INSERT INTO users (kakao_id, email, nickname, profile_image)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (kakao_id) DO UPDATE SET
			nickname = EXCLUDED.nickname,
			profile_image = EXCLUDED.profile_image,
			updated_at = NOW()
		RETURNING id, kakao_id, email, email_verified, nickname, profile_image, role, created_at, updated_at`

	var u user.User
	err := r.pool.QueryRow(ctx, query, kakaoID, email, nickname, profileImage).Scan(
		&u.ID, &u.KakaoID, &u.Email, &u.EmailVerified, &u.Nickname, &u.ProfileImage, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return user.User{}, fmt.Errorf("upsert user: %w", err)
	}
	return u, nil
}

func (r *UserRepository) FindByKakaoID(ctx context.Context, kakaoID int64) (user.User, bool, error) {
	query := `SELECT id, kakao_id, email, email_verified, nickname, profile_image, role, created_at, updated_at FROM users WHERE kakao_id = $1`

	var u user.User
	err := r.pool.QueryRow(ctx, query, kakaoID).Scan(
		&u.ID, &u.KakaoID, &u.Email, &u.EmailVerified, &u.Nickname, &u.ProfileImage, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return user.User{}, false, nil
		}
		return user.User{}, false, fmt.Errorf("find user by kakao id: %w", err)
	}
	return u, true, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (user.User, error) {
	query := `SELECT id, kakao_id, email, email_verified, nickname, profile_image, role, created_at, updated_at FROM users WHERE id = $1`

	var u user.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.KakaoID, &u.Email, &u.EmailVerified, &u.Nickname, &u.ProfileImage, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return user.User{}, fmt.Errorf("user not found")
		}
		return user.User{}, fmt.Errorf("find user: %w", err)
	}
	return u, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (user.User, error) {
	query := `SELECT id, kakao_id, email, email_verified, nickname, profile_image, role, created_at, updated_at FROM users WHERE email = $1`

	var u user.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.KakaoID, &u.Email, &u.EmailVerified, &u.Nickname, &u.ProfileImage, &u.Role, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return user.User{}, fmt.Errorf("user not found")
		}
		return user.User{}, fmt.Errorf("find user by email: %w", err)
	}
	return u, nil
}

func (r *UserRepository) UpdateEmail(ctx context.Context, userID string, email string) error {
	query := `UPDATE users SET email = $1, email_verified = true, updated_at = NOW() WHERE id = $2`
	tag, err := r.pool.Exec(ctx, query, email, userID)
	if err != nil {
		return fmt.Errorf("update email: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}
	return nil
}

func (r *UserRepository) DeleteByID(ctx context.Context, userID string) error {
	query := `DELETE FROM users WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}
	return nil
}
