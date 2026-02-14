package user

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository interface {
	UpsertByKakaoID(ctx context.Context, kakaoID int64, email, nickname, profileImage string) (User, error)
	FindByID(ctx context.Context, id string) (User, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) UpsertByKakaoID(ctx context.Context, kakaoID int64, email, nickname, profileImage string) (User, error) {
	query := `
		INSERT INTO users (kakao_id, email, nickname, profile_image)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (kakao_id) DO UPDATE SET
			email = EXCLUDED.email,
			nickname = EXCLUDED.nickname,
			profile_image = EXCLUDED.profile_image,
			updated_at = NOW()
		RETURNING id, kakao_id, email, nickname, profile_image, created_at, updated_at`

	var u User
	err := r.pool.QueryRow(ctx, query, kakaoID, email, nickname, profileImage).Scan(
		&u.ID, &u.KakaoID, &u.Email, &u.Nickname, &u.ProfileImage, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return User{}, fmt.Errorf("upsert user: %w", err)
	}
	return u, nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (User, error) {
	query := `SELECT id, kakao_id, email, nickname, profile_image, created_at, updated_at FROM users WHERE id = $1`

	var u User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.KakaoID, &u.Email, &u.Nickname, &u.ProfileImage, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return User{}, fmt.Errorf("user not found")
		}
		return User{}, fmt.Errorf("find user: %w", err)
	}
	return u, nil
}
