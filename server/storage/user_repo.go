package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ota/domain/user"
)

// CompleteSignupParams holds all data needed to atomically complete a signup.
type CompleteSignupParams struct {
	KakaoID         int64
	Email           string
	Nickname        string
	ProfileImageURL string
	AgreedTermIDs   []string
	SignupBonus     int // 0 means no bonus
}

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

// CompleteSignupTx atomically:
//  1. Upserts the user row
//  2. Inserts user_term_consent rows for each agreed term ID
//  3. (If SignupBonus > 0) Upserts user_points with the bonus amount
//  4. (If SignupBonus > 0) Inserts a coin_event audit record
//
// All four operations run in a single transaction. On any error the
// transaction is rolled back and no partial data is written.
func (r *UserRepository) CompleteSignupTx(ctx context.Context, p CompleteSignupParams) (user.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return user.User{}, fmt.Errorf("complete signup begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Upsert user
	var u user.User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (kakao_id, email, nickname, profile_image)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (kakao_id) DO UPDATE SET
			nickname      = EXCLUDED.nickname,
			profile_image = EXCLUDED.profile_image,
			updated_at    = NOW()
		RETURNING id, kakao_id, email, email_verified, nickname, profile_image, role, created_at, updated_at`,
		p.KakaoID, p.Email, p.Nickname, p.ProfileImageURL,
	).Scan(&u.ID, &u.KakaoID, &u.Email, &u.EmailVerified, &u.Nickname, &u.ProfileImage, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return user.User{}, fmt.Errorf("complete signup upsert user: %w", err)
	}

	// 2. Insert term consents
	for _, termID := range p.AgreedTermIDs {
		_, err = tx.Exec(ctx,
			`INSERT INTO user_term_consents (user_id, term_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			u.ID, termID,
		)
		if err != nil {
			return user.User{}, fmt.Errorf("complete signup insert consent for term %s: %w", termID, err)
		}
	}

	// 3 & 4. Signup bonus (skipped when zero)
	if p.SignupBonus > 0 {
		_, err = tx.Exec(ctx, `
			INSERT INTO user_points (user_id, points, updated_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (user_id) DO UPDATE SET
				points     = user_points.points + $2,
				updated_at = NOW()`,
			u.ID, p.SignupBonus,
		)
		if err != nil {
			return user.User{}, fmt.Errorf("complete signup add points: %w", err)
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO coin_events (user_id, amount, type, memo, actor_id) VALUES ($1, $2, $3, $4, NULL)`,
			u.ID, p.SignupBonus, "signup_bonus", "가입 보너스",
		)
		if err != nil {
			return user.User{}, fmt.Errorf("complete signup coin event: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return user.User{}, fmt.Errorf("complete signup commit: %w", err)
	}
	return u, nil
}
