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
			email = EXCLUDED.email,
			nickname = EXCLUDED.nickname,
			profile_image = EXCLUDED.profile_image,
			updated_at = NOW()
		RETURNING id, kakao_id, email, nickname, profile_image, created_at, updated_at`

	var u user.User
	err := r.pool.QueryRow(ctx, query, kakaoID, email, nickname, profileImage).Scan(
		&u.ID, &u.KakaoID, &u.Email, &u.Nickname, &u.ProfileImage, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return user.User{}, fmt.Errorf("upsert user: %w", err)
	}
	return u, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (user.User, error) {
	query := `SELECT id, kakao_id, email, nickname, profile_image, created_at, updated_at FROM users WHERE id = $1`

	var u user.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.KakaoID, &u.Email, &u.Nickname, &u.ProfileImage, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return user.User{}, fmt.Errorf("user not found")
		}
		return user.User{}, fmt.Errorf("find user: %w", err)
	}
	return u, nil
}

func (r *UserRepository) UpdateEmail(ctx context.Context, userID string, email string) error {
	query := `UPDATE users SET email = $1, updated_at = NOW() WHERE id = $2`
	tag, err := r.pool.Exec(ctx, query, email, userID)
	if err != nil {
		return fmt.Errorf("update email: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}
	return nil
}
